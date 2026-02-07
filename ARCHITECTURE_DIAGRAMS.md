# Roachprod Provider Architecture - Visual Guide

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         ROACHPROD CLI                            │
│                     (cmd/roachprod/main.go)                      │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                    ROACHPROD CORE LOGIC                          │
│                   (pkg/roachprod/roachprod.go)                   │
│                                                                   │
│  • InitProviders() - Initialize all cloud providers              │
│  • Create() - Create clusters                                    │
│  • Destroy() - Delete clusters                                   │
│  • List() - List clusters                                        │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                      PROVIDER REGISTRY                           │
│                      (vm.Providers map)                          │
│                                                                   │
│  vm.Providers = {                                                │
│    "aws":    &aws.Provider{},                                    │
│    "gce":    &gce.Provider{},                                    │
│    "azure":  &azure.Provider{},                                  │
│    "ibm":    &ibm.Provider{},                                    │
│    "ccloud": &ccloud.Provider{},  ← NEW!                         │
│    "local":  &local.Provider{}                                   │
│  }                                                                │
└───────────────────────────┬─────────────────────────────────────┘
                            │
            ┌───────────────┴───────────────┐
            ▼                               ▼
    ┌──────────────┐              ┌──────────────────┐
    │ vm.Provider  │              │  vm.Provider     │
    │  Interface   │              │  Interface       │
    │              │              │                  │
    │ • Create()   │              │  • Create()      │
    │ • Delete()   │              │  • Delete()      │
    │ • List()     │              │  • List()        │
    │ • Extend()   │              │  • Extend()      │
    │ • ...        │              │  • ...           │
    └──────┬───────┘              └────────┬─────────┘
           │                               │
           ▼                               ▼
   ┌───────────────┐            ┌───────────────────┐
   │ AWS Provider  │            │  Cockroach Cloud  │
   │               │            │    Provider       │
   │ • AWS SDK     │            │                   │
   │ • Terraform   │            │  • REST API       │
   │ • EC2 API     │            │  • API Client     │
   │               │            │  • Centralized    │
   └───────┬───────┘            └─────────┬─────────┘
           │                              │
           ▼                              ▼
   ┌───────────────┐            ┌──────────────────┐
   │   Amazon      │            │  Cockroach Cloud │
   │     EC2       │            │    Platform      │
   └───────────────┘            └──────────────────┘
```

## 2. Provider Initialization Flow

```
User runs: roachprod create my-cluster --clouds ccloud

         │
         ▼
┌─────────────────────┐
│  main.go            │
│  CLI Entry Point    │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  InitProviders()    │
│  in roachprod.go    │
└──────────┬──────────┘
           │
           │ For each provider:
           ├────────────────────────┐
           │                        │
           ▼                        ▼
    ┌──────────┐            ┌──────────────┐
    │aws.Init()│            │ccloud.Init() │
    └─────┬────┘            └──────┬───────┘
          │                        │
          │                        │ Check for:
          │                        │ • COCKROACH_CLOUD_API_KEY env var
          │                        │ • ccloud CLI in PATH
          │                        │
          │                        ▼
          │              ┌──────────────────────┐
          │              │ Auth Found?          │
          │              └──────┬───────────────┘
          │                     │
          │            Yes ◄────┴────► No
          │             │              │
          │             ▼              ▼
          │    ┌─────────────┐  ┌─────────────────┐
          │    │NewProvider()│  │Create Stub      │
          │    │             │  │ (Inactive)      │
          │    └──────┬──────┘  └────────┬────────┘
          │           │                  │
          │           ▼                  │
          │  ┌────────────────┐          │
          │  │Register Active │          │
          │  │   Provider     │          │
          │  └────────┬───────┘          │
          │           │                  │
          └───────────┴──────────────────┘
                      │
                      ▼
          ┌───────────────────────┐
          │ vm.Providers["ccloud"]│
          │   = provider instance │
          └───────────────────────┘
```

## 3. VM Creation Flow (Create Command)

```
User: roachprod create test-cluster --clouds ccloud -n 3

         │
         ▼
┌──────────────────────────┐
│ roachprod.Create()       │
│                          │
│ 1. Parse cluster name    │
│ 2. Get provider instance │
│ 3. Prepare options       │
└────────────┬─────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ cloud.CreateCluster()            │
│                                  │
│ • Validate options               │
│ • Select provider from --clouds  │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ ccloud.Provider.Create()         │
│                                  │
│ Input:                           │
│  • names: ["test-1","test-2"..] │
│  • opts: CreateOpts              │
│  • providerOpts: ProviderOpts    │
└────────────┬─────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ Call Cockroach Cloud API           │
│                                    │
│ POST /api/v1/clusters              │
│ {                                  │
│   "name": "test-cluster",          │
│   "nodes": 3,                      │
│   "machine_type": "n1-standard-4", │
│   "zones": ["us-east-1a"]          │
│ }                                  │
└────────────┬───────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ Cockroach Cloud Creates Cluster    │
│                                    │
│ Returns:                           │
│ {                                  │
│   "cluster_id": "abc123",          │
│   "nodes": [                       │
│     {                              │
│       "id": "node1",               │
│       "ip": "10.0.0.1"             │
│     },                             │
│     ...                            │
│   ]                                │
│ }                                  │
└────────────┬───────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ Map API Response to vm.VM structs  │
│                                    │
│ vms := []vm.VM{                    │
│   {                                │
│     Name: "test-1",                │
│     Provider: "ccloud",            │
│     ProviderID: "node1",           │
│     PublicIP: "10.0.0.1",          │
│     CreatedAt: time.Now(),         │
│   },                               │
│   ...                              │
│ }                                  │
└────────────┬───────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ Return vm.List to roachprod        │
└────────────┬───────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ roachprod saves cluster metadata   │
│ to ~/.roachprod/clusters/          │
└────────────────────────────────────┘
```

## 4. Provider Interface Implementation Map

```
┌─────────────────────────────────────────────────────────────┐
│                      vm.Provider Interface                   │
│                      (All methods required)                  │
└─────────────────────────────────────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         │                  │                  │
         ▼                  ▼                  ▼
┌────────────────┐  ┌────────────────┐  ┌──────────────────┐
│ AWS Provider   │  │ GCE Provider   │  │ CCloud Provider  │
├────────────────┤  ├────────────────┤  ├──────────────────┤
│✓ Create()      │  │✓ Create()      │  │✓ Create()        │
│✓ Delete()      │  │✓ Delete()      │  │✓ Delete()        │
│✓ List()        │  │✓ List()        │  │✓ List()          │
│✓ Reset()       │  │✓ Reset()       │  │✓ Reset()         │
│✓ Extend()      │  │✓ Extend()      │  │✓ Extend()        │
│✓ Grow()        │  │✓ Grow()        │  │✓ Grow()          │
│✓ Shrink()      │  │✓ Shrink()      │  │✓ Shrink()        │
│✓ AddLabels()   │  │✓ AddLabels()   │  │✓ AddLabels()     │
│✓ CreateVolume()│  │✓ CreateVolume()│  │✓ CreateVolume()  │
│✓ SpotVMs: YES  │  │✓ SpotVMs: YES  │  │✗ SpotVMs: NO     │
│✗ Centralized   │  │✗ Centralized   │  │✓ Centralized     │
└────────────────┘  └────────────────┘  └──────────────────┘
```

## 5. File Organization

```
pkg/roachprod/vm/ccloud/
│
├── provider.go              ← Core provider implementation
│   ├── type Provider struct
│   ├── func Init()
│   ├── func NewProvider()
│   ├── func Create()
│   ├── func Delete()
│   ├── func List()
│   └── ... (all interface methods)
│
├── provider_opts.go         ← Provider-specific options
│   ├── type ProviderOpts struct
│   ├── func ConfigureCreateFlags()
│   └── func ConfigureClusterFlags()
│
├── provider_test.go         ← Unit tests
│   ├── TestProviderInterface()
│   ├── TestProviderActive()
│   └── TestNewProvider()
│
├── BUILD.bazel              ← Bazel build configuration
│   ├── go_library(...)
│   └── go_test(...)
│
└── README.md                ← Provider documentation
    ├── Overview
    ├── Setup Instructions
    ├── Usage Examples
    └── Troubleshooting
```

## 6. Data Flow: Creating a 3-Node Cluster

```
┌──────────┐
│   User   │
└────┬─────┘
     │
     │ $ roachprod create my-cluster --clouds ccloud -n 3
     ▼
┌─────────────────────────────────────────────────────────┐
│ Step 1: Parse Command & Validate                        │
│ • cluster_name = "my-cluster"                           │
│ • cloud = "ccloud"                                      │
│ • num_nodes = 3                                         │
└────┬────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────┐
│ Step 2: Get Provider                                    │
│ • provider = vm.Providers["ccloud"]                     │
│ • Check provider.Active() == true                       │
└────┬────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────┐
│ Step 3: Call provider.Create()                          │
│ • names = ["my-cluster-1", "my-cluster-2", ...]         │
│ • opts = CreateOpts{Lifetime: 12h, ...}                 │
│ • providerOpts = ProviderOpts{MachineType: ...}         │
└────┬────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────┐
│ Step 4: Cockroach Cloud API Call                        │
│                                                          │
│  HTTP POST https://cockroachlabs.cloud/api/v1/clusters  │
│  Header: Authorization: Bearer <API_KEY>                │
│  Body: {                                                │
│    "name": "my-cluster",                                │
│    "num_nodes": 3,                                      │
│    "machine_type": "n1-standard-4",                     │
│    "region": "us-east-1"                                │
│  }                                                      │
└────┬────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────┐
│ Step 5: API Response                                    │
│                                                          │
│  {                                                      │
│    "cluster_id": "clstr-abc123",                        │
│    "status": "creating",                                │
│    "nodes": [                                           │
│      {"id": "n1", "ip": "34.73.1.2"},                   │
│      {"id": "n2", "ip": "34.73.1.3"},                   │
│      {"id": "n3", "ip": "34.73.1.4"}                    │
│    ]                                                    │
│  }                                                      │
└────┬────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────┐
│ Step 6: Map to vm.VM Objects                            │
│                                                          │
│  vms = [                                                │
│    vm.VM{                                               │
│      Name: "my-cluster-1",                              │
│      Provider: "ccloud",                                │
│      ProviderID: "n1",                                  │
│      PublicIP: "34.73.1.2",                             │
│      RemoteUser: "ubuntu",                              │
│      CreatedAt: 2026-02-07T16:00:00Z                    │
│    },                                                   │
│    ... (2 more VMs)                                     │
│  ]                                                      │
└────┬────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────┐
│ Step 7: Save Cluster Metadata                           │
│                                                          │
│  File: ~/.roachprod/clusters/my-cluster.json            │
│  {                                                      │
│    "name": "my-cluster",                                │
│    "provider": "ccloud",                                │
│    "vms": [...],                                        │
│    "created_at": "2026-02-07T16:00:00Z"                 │
│  }                                                      │
└────┬────────────────────────────────────────────────────┘
     │
     ▼
┌──────────┐
│  Success │
│  Message │
└──────────┘
```

## 7. Comparison: Centralized vs Distributed Providers

```
┌─────────────────────────────────────────────────────────────────┐
│              CENTRALIZED PROVIDER (Cockroach Cloud)              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Roachprod        API          Cockroach Cloud                  │
│  ┌──────┐       ┌───┐         ┌────────────┐                   │
│  │Client│◄─────►│API│◄───────►│  Platform  │                   │
│  └──────┘       └───┘         │            │                   │
│      │                        │ • Clusters │                   │
│      │                        │ • VMs      │                   │
│      │                        │ • State    │                   │
│      ▼                        └────────────┘                   │
│  ┌──────────┐                                                   │
│  │Local     │  ← Lightweight cache only                        │
│  │Cache     │                                                   │
│  └──────────┘                                                   │
│                                                                  │
│  ✓ Single source of truth (cloud platform)                     │
│  ✓ No local state management complexity                        │
│  ✓ Always in sync                                              │
│  ✗ Requires network for all operations                         │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│              DISTRIBUTED PROVIDER (AWS, GCP)                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Roachprod        API          Cloud Provider                   │
│  ┌──────┐       ┌───┐         ┌────────────┐                   │
│  │Client│◄─────►│API│◄───────►│    EC2     │                   │
│  └───┬──┘       └───┘         │  Service   │                   │
│      │                        └────────────┘                   │
│      ▼                                                           │
│  ┌──────────┐                                                   │
│  │~/.roachprod/ ← Full state stored locally                     │
│  │clusters/ │                                                   │
│  │  └─ my-cluster.json                                          │
│  └──────────┘                                                   │
│                                                                  │
│  ✓ Works offline (list cached clusters)                        │
│  ✓ Faster local operations                                     │
│  ✗ State synchronization complexity                            │
│  ✗ Can get out of sync with cloud                              │
└─────────────────────────────────────────────────────────────────┘
```

## 8. Key Design Principles

```
┌────────────────────────────────────────────────────────┐
│ 1. ABSTRACTION                                         │
│    Hide cloud-specific details behind vm.Provider      │
│    ┌──────────┐                                        │
│    │ Roachprod│ → Doesn't know if VM is on AWS/GCP/etc │
│    └──────────┘                                        │
└────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────┐
│ 2. UNIFORMITY                                          │
│    Same operations across all providers               │
│    Create() → Works the same for AWS, GCP, CCloud     │
└────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────┐
│ 3. EXTENSIBILITY                                       │
│    Easy to add new providers                          │
│    1. Implement vm.Provider interface                 │
│    2. Register in InitProviders()                     │
│    3. Done!                                           │
└────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────┐
│ 4. GRACEFUL DEGRADATION                                │
│    Providers work even if not configured              │
│    ┌─────────┐                                        │
│    │No API   │ → Provider registered as "stub"        │
│    │Key Set  │    Shows helpful error messages        │
│    └─────────┘                                        │
└────────────────────────────────────────────────────────┘
```

This visual guide illustrates how the Cockroach Cloud provider integrates
seamlessly into the roachprod framework's extensible architecture.
