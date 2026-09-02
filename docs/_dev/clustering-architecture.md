# OtterMQ Clustering Architecture

## Overview

This document outlines a high-level architecture for implementing clustering support in OtterMQ while maintaining full AMQP 0.9.1 protocol compatibility.

## Core Components

### 1. Node Discovery & Membership

**Gossip Protocol for Node Discovery**
- Implement gossip-based protocol similar to RabbitMQ's use of Erlang's `pg2`
- Membership registry tracking all active cluster members
- Periodic health checking and failure detection
- Split-brain prevention mechanisms
- Configurable gossip interval and failure thresholds

**Responsibilities:**
- Announce node presence to cluster
- Maintain list of active cluster members
- Detect and handle node failures
- Prevent network partition issues

### 2. Distributed Metadata Store

**Cluster-Wide Topology Replication**
- Synchronize exchange declarations across all nodes
- Replicate queue metadata (properties, bindings)
- Share binding information cluster-wide
- Maintain consistent virtual host configurations

**Integration with Memento WAL Engine**
- Use WAL-based replication for metadata changes
- Event sourcing for topology updates
- Snapshot + log tail for new node bootstrapping
- Conflict resolution for concurrent updates

**Key Data to Replicate:**
- Exchange definitions (name, type, durability, arguments)
- Queue definitions (name, durability, arguments, location)
- Binding relationships
- Virtual host configurations
- User permissions and access control

### 3. Message Distribution Layer

**Queue Location Registry**
- Track which node hosts which queue (master node)
- Map queue names to node addresses
- Handle queue migration during rebalancing
- Support lookup by queue name or pattern

**Message Routing Between Nodes**
- Forward messages to queue's home node
- Optimize routing to minimize hops
- Handle transient routing during topology changes
- Support priority-based forwarding

**Client Connection Routing**
- Balance client connections across nodes
- Provide cluster endpoint abstraction
- Support client affinity to specific nodes
- Handle connection failover transparently

## Queue Distribution Strategies

### Strategy 1: Master/Replica Model (Initial Implementation)

**Characteristics:**
- Each queue has a primary node (master)
- Optional mirror nodes for high availability
- All writes go through master node
- Reads can be served from mirrors (policy-dependent)
- Failover promotes mirror to master on failure

**Pros:**
- Simpler to implement
- Strong consistency guarantees
- Compatible with AMQP message ordering
- Well-understood failure scenarios

**Cons:**
- Master node can become bottleneck
- Requires coordination for failover
- Network overhead for replication

### Strategy 2: Distributed Queue Model (Future Enhancement)

**Characteristics:**
- Queue partitioned across multiple nodes
- Consistent hashing for message placement
- Each partition independently managed
- Parallel consumption from multiple nodes

**Pros:**
- Higher throughput potential
- Better resource utilization
- Natural load balancing

**Cons:**
- More complex implementation
- Message ordering challenges
- Requires careful partition key design

## Message Flow in Cluster

### Publishing Flow

```
Client publishes to Exchange
    ↓
Node A receives publish
    ↓
Routing determines target queue
    ↓
Queue is on Node B (different node)
    ↓
Node A forwards message to Node B
    ↓
Node B writes to queue
    ↓
Node B sends confirmation back to Node A
    ↓
Node A confirms to client
```

### Consumption Flow

```
Consumer on Node C subscribes to queue
    ↓
Queue is on Node B (different node)
    ↓
Node C registers remote consumer with Node B
    ↓
Node B delivers messages to Node C
    ↓
Node C forwards to actual consumer
    ↓
Consumer acknowledges
    ↓
Node C forwards ACK to Node B
    ↓
Node B removes message from queue
```

### Transaction Coordination

```
Client starts TX on Node A
    ↓
Operations may touch queues on Nodes B, C
    ↓
Node A tracks all participating nodes
    ↓
Client commits TX
    ↓
Node A coordinates 2PC with Nodes B, C
    ↓
All nodes prepare (vote yes/no)
    ↓
Node A decides commit/abort
    ↓
All nodes execute decision
    ↓
Node A confirms to client
```

## Implementation Phases

### Phase 1: Foundation (Months 1-2)

**Goals:**
- Implement Memento WAL for reliable state replication
- Add cluster configuration system
- Build node discovery and membership management
- Create inter-node RPC/messaging protocol

**Deliverables:**
- `pkg/cluster/membership/` - Gossip protocol implementation
- `pkg/cluster/rpc/` - Inter-node communication
- `internal/core/broker/cluster_config.go` - Configuration
- WAL-based metadata replication

**Testing:**
- 2-node cluster basic connectivity
- Node join/leave scenarios
- Failure detection accuracy
- Metadata synchronization correctness

### Phase 2: Metadata Synchronization (Months 3-4)

**Goals:**
- Replicate exchange/queue declarations cluster-wide
- Synchronize bindings across all nodes
- Handle topology changes (node add/remove)
- Implement consistent queue location tracking

**Deliverables:**
- `internal/core/broker/cluster_metadata.go` - Metadata sync
- `internal/core/broker/vhost/cluster_exchange.go` - Distributed exchanges
- `internal/core/broker/vhost/cluster_queue.go` - Queue registry
- Topology change handlers

**Testing:**
- Declare exchange on Node A, verify on Node B
- Create queue on one node, bind from another
- Node failure with metadata recovery
- Concurrent topology modifications

### Phase 3: Message Routing (Months 5-6)

**Goals:**
- Implement queue location tracking
- Build message forwarding between nodes
- Add consumer balancing logic
- Handle connection affinity

**Deliverables:**
- `internal/core/broker/cluster_router.go` - Message forwarding
- `internal/core/broker/vhost/remote_queue.go` - Queue proxy
- Consumer registration across nodes
- Performance optimizations (batching, pipelining)

**Testing:**
- Publish to Node A, consume from Node B
- Multi-hop message delivery
- Consumer failover scenarios
- Throughput benchmarks

### Phase 4: High Availability (Months 7-8)

**Goals:**
- Implement queue mirroring/replication
- Add automatic failover mechanisms
- Handle network partitions gracefully
- Implement split-brain resolution

**Deliverables:**
- `internal/core/broker/vhost/queue_mirror.go` - Replication
- `pkg/cluster/consensus/` - Leader election (Raft)
- Partition detection and healing
- Quorum-based decision making

**Testing:**
- Master node failure with mirror promotion
- Network partition scenarios
- Split-brain resolution
- Data consistency validation

## Technical Decisions

### Communication Protocol

**Option 1: gRPC (Recommended)**
- Type-safe with Protocol Buffers
- Built-in streaming support
- Good performance characteristics
- Wide language support for future clients

**Option 2: Custom Binary Protocol**
- Maximum performance control
- Minimal overhead
- Requires more maintenance effort

**Option 3: AMQP Self-Hosting**
- Dogfooding our own protocol
- Simpler mental model
- May have performance limitations

**Decision:** Start with gRPC for inter-node communication, evaluate custom protocol if performance requires it.

### Consensus Algorithm

**Metadata Consistency: Raft**
- Leader election for metadata updates
- Strong consistency for topology changes
- Well-tested implementations available (etcd/raft)

**Membership Management: Gossip**
- Eventually consistent view of cluster
- Lower coordination overhead
- Natural fit for health checking

### State Synchronization Strategy

**Event Sourcing via Memento WAL**
- All state changes are events in the log
- Replay log to reconstruct state
- Natural audit trail
- Supports time-travel debugging

**Snapshot + Log Tail**
- New nodes start with snapshot
- Apply log from snapshot point forward
- Balance between recovery speed and storage

**Conflict Resolution**
- Last-write-wins for configuration updates
- Vector clocks for causality tracking
- Application-level conflict handlers for special cases

## Challenges and Solutions

### Challenge 1: Network Partitions

**Problem:** Cluster splits into isolated groups

**Solutions:**
- Quorum-based writes (majority required)
- Readonly mode for minority partitions
- Automatic partition healing when connectivity restored
- Client redirection to majority partition

### Challenge 2: Message Ordering

**Problem:** Cross-node routing can disorder messages

**Solutions:**
- Keep related messages on same node (sharding key)
- Sequence numbers for detection
- Per-producer ordering guarantees
- Consumer-side reordering if needed

### Challenge 3: Transaction Coordination

**Problem:** Distributed transactions are expensive

**Solutions:**
- Optimize common case (single-queue transactions)
- Async replication for non-transactional publishes
- Two-phase commit only when necessary
- Transaction timeout and recovery

### Challenge 4: Performance Overhead

**Problem:** Network latency impacts throughput

**Solutions:**
- Message batching for cross-node transfers
- Compression for large payloads
- Connection pooling between nodes
- Strategic queue placement (locality)

### Challenge 5: Client Transparency

**Problem:** Should clients know about clustering?

**Solution:**
- Cluster appears as single endpoint
- Client libraries need cluster-aware connection strings
- Automatic failover in client libraries
- Optional client-side load balancing

## Integration with Current Architecture

### VHost Layer Integration

```go
// Current: Single-node VHost
type VHost struct {
    exchanges map[string]*Exchange
    queues    map[string]*Queue
    // ...
}

// Cluster-aware: Add distribution layer
type VHost struct {
    exchanges       map[string]*Exchange
    queues          map[string]*Queue
    queueLocations  *QueueLocationRegistry  // NEW: track queue locations
    remoteQueues    *RemoteQueueProxy       // NEW: proxy to other nodes
    clusterRouter   *ClusterRouter          // NEW: message forwarding
    // ...
}
```

### MessageController Enhancement

```go
// Implement cluster-aware message routing
type ClusterMessageController struct {
    localController  MessageController
    clusterRouter    *ClusterRouter
    queueRegistry    *QueueLocationRegistry
}

func (c *ClusterMessageController) Publish(exchange, routingKey string, msg *Message) error {
    targetQueues := c.localController.RouteMessage(exchange, routingKey)
    
    for _, queueName := range targetQueues {
        location := c.queueRegistry.GetQueueLocation(queueName)
        
        if location.IsLocal() {
            // Local delivery
            c.localController.DeliverToQueue(queueName, msg)
        } else {
            // Remote delivery
            c.clusterRouter.ForwardMessage(location.NodeID, queueName, msg)
        }
    }
    return nil
}
```

### Queue Interface Extension

```go
// Add cluster awareness to queue interface
type Queue interface {
    // Existing methods
    Push(msg *Message) error
    Pop() (*Message, error)
    // ...
    
    // NEW: Cluster methods
    GetLocation() NodeID
    SetLocation(nodeID NodeID)
    IsMirrored() bool
    GetMirrors() []NodeID
}
```

## Quick Win Path: Non-HA Clustering

### Scope
Implement distributed capacity without high availability complexity.

### Features
1. **Static Cluster Configuration**
   - Predefined list of cluster nodes
   - No dynamic discovery (add later)
   - Simple configuration file

2. **Single-Master Queues**
   - Each queue lives on exactly one node
   - No replication or mirroring
   - Clear ownership model

3. **Automatic Message Forwarding**
   - Publish to any node, automatically forwarded to queue's home node
   - Transparent to clients
   - Simple routing logic

4. **Unified Cluster View**
   - Client sees cluster as single entity
   - Management UI shows all nodes
   - Consistent topology across nodes

### Benefits
- Horizontal scaling without HA complexity
- Foundation for later HA features
- Simpler testing and debugging
- Faster time to market

### Limitations (Addressed in Later Phases)
- Queue unavailable if hosting node fails
- Manual queue migration
- No automatic load balancing
- Single point of failure per queue

## Configuration Example

```yaml
# cluster.yaml
cluster:
  enabled: true
  node:
    id: "ottermq-1"
    host: "192.168.1.10"
    amqp_port: 5672
    cluster_port: 5673  # Inter-node communication
    
  peers:
    - id: "ottermq-2"
      host: "192.168.1.11"
      cluster_port: 5673
    - id: "ottermq-3"
      host: "192.168.1.12"
      cluster_port: 5673
      
  gossip:
    interval: "1s"
    failure_timeout: "10s"
    
  replication:
    strategy: "none"  # "none" | "async" | "sync"
    min_replicas: 0
    
  queue_placement:
    strategy: "least-loaded"  # "round-robin" | "least-loaded" | "manual"
```

## Success Metrics

### Performance Metrics
- Message throughput scales linearly with nodes (0.8x ideal)
- Cross-node latency < 5ms (local network)
- Metadata sync latency < 100ms
- Failover time < 5s for HA mode

### Reliability Metrics
- Zero message loss during normal operation
- Zero message loss during node failure (HA mode)
- Successful partition healing within 30s
- No split-brain scenarios

### Operational Metrics
- Simple cluster setup (< 10 steps)
- Clear monitoring and alerting
- Easy node addition/removal
- Automated failure recovery

## Future Enhancements

### Advanced Features (Post-V1)
- Geographic distribution (multi-datacenter)
- Federated clusters (cross-cluster routing)
- Dynamic queue migration for load balancing
- Automatic capacity scaling
- Enhanced monitoring and observability

### Performance Optimizations
- RDMA support for high-speed networks
- Kernel bypass for ultra-low latency
- GPU acceleration for message processing
- Advanced compression algorithms

## References

- **RabbitMQ Clustering**: https://www.rabbitmq.com/clustering.html
- **Raft Consensus**: https://raft.github.io/
- **Gossip Protocol**: https://en.wikipedia.org/wiki/Gossip_protocol
- **Two-Phase Commit**: https://en.wikipedia.org/wiki/Two-phase_commit_protocol
