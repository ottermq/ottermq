# Kafka-Inspired Implementation Patterns for OtterMQ

## Overview

This document outlines performance and scalability patterns from Apache Kafka that can be adopted by OtterMQ to improve internal implementation while maintaining full AMQP 0.9.1 protocol compatibility.

**Key Principle:** All these patterns operate below the AMQP protocol layer. Clients see standard AMQP 0.9.1 behavior, but internally OtterMQ uses battle-tested storage and delivery optimizations from Kafka.

## Pattern 1: Log-Structured Storage

### Kafka's Approach

Messages are stored in append-only log segments on disk with sequential writes.

**Key Characteristics:**
- Immutable log segments
- Sequential disk writes (extremely fast)
- Segment rotation at size/time limits
- Index files for efficient lookup

### OtterMQ Implementation

**Current State:**
```
In-memory channels → Periodic JSON snapshots
Problems: Memory limits, slow recovery, data loss risk
```

**Proposed Architecture:**
```
/data/vhosts/%2F/queues/myqueue/
├── 00000000000000000000.log     # Segment 1 (sealed)
├── 00000000000000000000.index   # Index for segment 1
├── 00000000000000001000.log     # Segment 2 (sealed)
├── 00000000000000001000.index   # Index for segment 2
├── 00000000000000002000.log     # Segment 3 (active)
└── 00000000000000002000.index   # Index for segment 3
```

**Implementation Details:**

```go
// pkg/persistence/implementations/wal/segment.go
type Segment struct {
    path       string
    file       *os.File
    index      *Index
    baseOffset uint64
    position   int64
    size       int64
    sealed     bool
}

func (s *Segment) Append(msg *Message) (uint64, error) {
    if s.sealed {
        return 0, ErrSegmentSealed
    }
    
    // Serialize message
    data, err := msg.Marshal()
    if err != nil {
        return 0, err
    }
    
    // Write to log
    offset := s.baseOffset + uint64(s.position)
    _, err = s.file.WriteAt(data, s.position)
    if err != nil {
        return 0, err
    }
    
    // Update index
    s.index.Add(offset, s.position, len(data))
    s.position += int64(len(data))
    
    return offset, nil
}

func (s *Segment) Read(offset uint64) (*Message, error) {
    // Lookup position in index
    position, size, err := s.index.Lookup(offset)
    if err != nil {
        return nil, err
    }
    
    // Read from log
    data := make([]byte, size)
    _, err = s.file.ReadAt(data, position)
    if err != nil {
        return nil, err
    }
    
    // Deserialize
    return UnmarshalMessage(data)
}
```

**Queue Integration:**

```go
// internal/core/broker/vhost/queue_wal.go
type WALQueue struct {
    name           string
    segments       []*Segment
    activeSegment  *Segment
    nextOffset     uint64
    
    // Configuration
    segmentSize    int64  // e.g., 1GB
    maxSegments    int    // Retention policy
}

func (q *WALQueue) Push(msg *Message) error {
    // Check if segment is full
    if q.activeSegment.Size() >= q.segmentSize {
        q.rotateSegment()
    }
    
    // Append to active segment
    offset, err := q.activeSegment.Append(msg)
    if err != nil {
        return err
    }
    
    q.nextOffset = offset + 1
    return nil
}

func (q *WALQueue) rotateSegment() error {
    // Seal current segment
    q.activeSegment.Seal()
    
    // Create new segment
    newSegment, err := NewSegment(q.segmentPath(q.nextOffset), q.nextOffset)
    if err != nil {
        return err
    }
    
    q.segments = append(q.segments, newSegment)
    q.activeSegment = newSegment
    
    // Clean old segments if needed
    q.cleanupOldSegments()
    
    return nil
}
```

### Benefits

1. **Persistence by Default**: Every message is on disk without performance penalty
2. **Fast Recovery**: Replay from last checkpoint, no full rebuild
3. **Predictable Performance**: Sequential writes are fast and consistent
4. **Natural Retention**: Time and size-based cleanup
5. **Foundation for Memento WAL**: Core building block for your planned WAL engine

### AMQP Compatibility

✅ **Zero protocol impact** - purely internal storage layer change

### Performance Characteristics

- **Write**: O(1) append operation, ~500MB/s sequential writes
- **Read**: O(log n) index lookup + O(1) read
- **Memory**: Minimal, OS page cache handles caching
- **Disk**: Sequential writes, SSD-friendly

## Pattern 2: Consumer Offset Tracking

### Kafka's Approach

Each consumer maintains an offset pointer into the log. Multiple consumers can read the same message independently.

**Key Characteristics:**
- Offsets are simple integers (cheap to manage)
- Messages aren't deleted immediately after consumption
- Natural support for replay and redelivery
- Consumer groups share offset progress

### OtterMQ Implementation

**Current State:**
```
Message delivered → ACK received → Message deleted
Problem: Can't redeliver to multiple consumers efficiently
```

**Proposed Architecture:**

```go
// internal/core/broker/vhost/consumer_offset.go
type ConsumerOffset struct {
    ConsumerTag string
    VHost       string
    Queue       string
    Offset      uint64
    LastAck     time.Time
    Redelivered bool
}

type OffsetTracker struct {
    offsets map[string]*ConsumerOffset  // consumerTag -> offset
    mu      sync.RWMutex
}

func (t *OffsetTracker) GetOffset(consumerTag string) uint64 {
    t.mu.RLock()
    defer t.mu.RUnlock()
    
    if offset, ok := t.offsets[consumerTag]; ok {
        return offset.Offset
    }
    return 0  // Start from beginning
}

func (t *OffsetTracker) Commit(consumerTag string, offset uint64) {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    if existing, ok := t.offsets[consumerTag]; ok {
        existing.Offset = offset
        existing.LastAck = time.Now()
    } else {
        t.offsets[consumerTag] = &ConsumerOffset{
            ConsumerTag: consumerTag,
            Offset:      offset,
            LastAck:     time.Now(),
        }
    }
}
```

**Queue Integration:**

```go
// internal/core/broker/vhost/queue_offset.go
type OffsetBasedQueue struct {
    *WALQueue
    offsetTracker  *OffsetTracker
    minRetainOffset uint64  // Lowest offset still needed
}

func (q *OffsetBasedQueue) Deliver(consumerTag string) (*Message, uint64, error) {
    // Get consumer's current offset
    offset := q.offsetTracker.GetOffset(consumerTag)
    
    // Read message at offset
    msg, err := q.ReadAt(offset)
    if err != nil {
        return nil, 0, err
    }
    
    return msg, offset, nil
}

func (q *OffsetBasedQueue) Acknowledge(consumerTag string, deliveryTag uint64, multiple bool) error {
    if multiple {
        // ACK all messages up to and including deliveryTag
        q.offsetTracker.Commit(consumerTag, deliveryTag+1)
    } else {
        // ACK single message
        currentOffset := q.offsetTracker.GetOffset(consumerTag)
        if deliveryTag == currentOffset {
            q.offsetTracker.Commit(consumerTag, deliveryTag+1)
        }
    }
    
    // Update minimum retain offset
    q.updateMinRetainOffset()
    
    return nil
}

func (q *OffsetBasedQueue) updateMinRetainOffset() {
    minOffset := uint64(math.MaxUint64)
    
    for _, offset := range q.offsetTracker.GetAllOffsets() {
        if offset.Offset < minOffset {
            minOffset = offset.Offset
        }
    }
    
    q.minRetainOffset = minOffset
}
```

**AMQP Delivery Tag Mapping:**

```go
// Delivery tags map directly to offsets
type DeliveryTagMapper struct {
    channelID uint16
}

func (m *DeliveryTagMapper) OffsetToDeliveryTag(offset uint64) uint64 {
    // Delivery tag is just the offset
    // Channel ID is already tracked separately in AMQP
    return offset
}

func (m *DeliveryTagMapper) DeliveryTagToOffset(deliveryTag uint64) uint64 {
    return deliveryTag
}
```

### Benefits

1. **Efficient Redelivery**: Don't need message copies for redelivery
2. **Message Replay**: Support `basic.recover` naturally
3. **Multiple Consumers**: Each consumer tracks own position independently
4. **Reduced Memory**: Messages stored once, consumed many times
5. **Audit Trail**: Keep messages for debugging/compliance

### AMQP Compatibility

✅ **Transparent mapping** - delivery tags are offsets internally
- `basic.deliver` sends offset as delivery tag
- `basic.ack` commits offset progress
- `basic.recover` resets offset to last committed
- `basic.nack` allows offset rollback

### Performance Characteristics

- **Memory**: O(consumers) instead of O(messages * consumers)
- **Redelivery**: O(1) offset reset vs O(n) message copy
- **Cleanup**: Batch delete old segments vs per-message deletion

## Pattern 3: Batched Writes & Group Commit

### Kafka's Approach

Accumulate multiple messages before flushing to disk, amortizing fsync cost across many messages.

### OtterMQ Implementation

```go
// pkg/persistence/implementations/wal/batch_writer.go
type BatchWriter struct {
    buffer      []*Message
    bufferSize  int
    maxSize     int
    maxWait     time.Duration
    
    flushChan   chan struct{}
    flushTimer  *time.Timer
    
    mu          sync.Mutex
}

func NewBatchWriter(maxSize int, maxWait time.Duration) *BatchWriter {
    bw := &BatchWriter{
        buffer:    make([]*Message, 0, maxSize),
        maxSize:   maxSize,
        maxWait:   maxWait,
        flushChan: make(chan struct{}, 1),
    }
    
    go bw.flushLoop()
    return bw
}

func (bw *BatchWriter) Add(msg *Message) chan error {
    bw.mu.Lock()
    defer bw.mu.Unlock()
    
    errChan := make(chan error, 1)
    
    bw.buffer = append(bw.buffer, msg)
    bw.bufferSize++
    
    // Start timer on first message
    if bw.bufferSize == 1 {
        bw.flushTimer = time.AfterFunc(bw.maxWait, func() {
            bw.flushChan <- struct{}{}
        })
    }
    
    // Flush if buffer full
    if bw.bufferSize >= bw.maxSize {
        bw.flushTimer.Stop()
        bw.flushChan <- struct{}{}
    }
    
    return errChan
}

func (bw *BatchWriter) flushLoop() {
    for range bw.flushChan {
        bw.flush()
    }
}

func (bw *BatchWriter) flush() error {
    bw.mu.Lock()
    toFlush := bw.buffer
    bw.buffer = make([]*Message, 0, bw.maxSize)
    bw.bufferSize = 0
    bw.mu.Unlock()
    
    if len(toFlush) == 0 {
        return nil
    }
    
    // Write all messages in batch
    for _, msg := range toFlush {
        if err := bw.segment.Append(msg); err != nil {
            return err
        }
    }
    
    // Single fsync for all messages
    return bw.segment.Sync()
}
```

**Configuration Options:**

```go
// config/persistence.go
type PersistenceConfig struct {
    // Batch settings
    BatchMaxSize     int           // e.g., 100 messages
    BatchMaxWait     time.Duration // e.g., 10ms
    
    // Durability modes
    FlushMode        string        // "immediate", "batch", "async"
    
    // Per-message overrides
    RespectPersistent bool         // delivery-mode=2 forces immediate flush
}
```

**Queue Integration:**

```go
func (q *WALQueue) Push(msg *Message) error {
    // Check flush mode
    forceImmediate := q.config.RespectPersistent && msg.DeliveryMode == 2
    
    if forceImmediate {
        // Synchronous write for persistent messages
        return q.segment.AppendSync(msg)
    }
    
    // Batched write
    errChan := q.batchWriter.Add(msg)
    return <-errChan  // Wait for batch flush
}
```

### Benefits

1. **Higher Throughput**: Amortize fsync cost (1 sync per 100 messages vs 100 syncs)
2. **Better Disk Utilization**: Larger sequential writes
3. **Configurable Tradeoff**: Balance durability vs performance
4. **Transaction Friendly**: Batch commit aligns with TX semantics

### AMQP Compatibility

✅ **Respects message properties**:
- `delivery-mode=2` (persistent) can trigger immediate flush
- `delivery-mode=1` (transient) uses batching
- Transaction commits force immediate flush
- Synchronous RPC-style publishes wait for batch flush

### Performance Impact

- **Throughput**: 5-10x improvement for small messages
- **Latency**: +10ms average (configurable wait time)
- **Disk I/O**: Reduced by 90%+ (batch size dependent)

## Pattern 4: Zero-Copy Message Transfer

### Kafka's Approach

Use `sendfile()` syscall to transfer data directly from disk to network socket, avoiding userspace copies.

### OtterMQ Implementation

```go
// internal/core/broker/vhost/zerocopy.go
package vhost

import (
    "os"
    "syscall"
)

type ZeroCopyDelivery struct {
    queue      *WALQueue
    segment    *Segment
    connection *Connection
}

func (d *ZeroCopyDelivery) DeliverMessage(offset uint64) error {
    // Get segment file and position
    segment, position, size := d.queue.LocateMessage(offset)
    
    // Open segment file
    file, err := os.Open(segment.Path())
    if err != nil {
        return err
    }
    defer file.Close()
    
    // Get socket file descriptor
    conn := d.connection.TCPConn()
    socketFD := int(conn.File().Fd())
    
    // Zero-copy transfer from file to socket
    // This happens in kernel space, no userspace copy
    offset64 := int64(position)
    _, err = syscall.Sendfile(socketFD, int(file.Fd()), &offset64, size)
    
    return err
}
```

**Hybrid Approach (Small vs Large Messages):**

```go
func (d *ZeroCopyDelivery) Deliver(msg *Message) error {
    const ZEROCOPY_THRESHOLD = 64 * 1024  // 64KB
    
    if msg.Size() < ZEROCOPY_THRESHOLD {
        // Small message: traditional delivery
        return d.deliverTraditional(msg)
    }
    
    // Large message: zero-copy
    return d.deliverZeroCopy(msg.Offset)
}

func (d *ZeroCopyDelivery) deliverTraditional(msg *Message) error {
    // Encode AMQP frame
    frame := encodeBasicDeliver(msg)
    
    // Write to socket
    return d.connection.WriteFrame(frame)
}
```

**Platform Compatibility:**

```go
// +build linux

func (d *ZeroCopyDelivery) sendfile(dst, src int, offset *int64, count int) error {
    _, err := syscall.Sendfile(dst, src, offset, count)
    return err
}

// +build !linux

func (d *ZeroCopyDelivery) sendfile(dst, src int, offset *int64, count int) error {
    // Fallback: traditional copy
    return d.deliverTraditional(msg)
}
```

### Benefits

1. **CPU Savings**: 2-3x reduction in CPU usage for large messages
2. **Higher Throughput**: 2-3x improvement for throughput-bound workloads
3. **Better Cache Utilization**: Keep L1/L2 cache for hot data
4. **Lower GC Pressure**: Fewer allocations and copies

### AMQP Compatibility

✅ **Transparent optimization** - works below protocol layer
- AMQP frames are sent normally
- Only the payload transfer is optimized
- Fallback to traditional copy if not supported

### When to Use

- Large messages (> 64KB recommended threshold)
- High throughput scenarios
- CPU-bound brokers
- Linux/Unix platforms (best support)

### Performance Characteristics

- **Small messages (< 1KB)**: No benefit, overhead from syscall
- **Medium messages (1-64KB)**: Marginal benefit
- **Large messages (> 64KB)**: Significant benefit (2-3x)

## Pattern 5: Page Cache Utilization

### Kafka's Approach

Rely heavily on OS page cache instead of application-managed memory. Let the OS handle memory pressure.

### OtterMQ Implementation

**Current State:**
```go
// Large in-memory channel buffer
type Queue struct {
    buffer chan *Message  // e.g., 100,000 messages in RAM
}
```

**Proposed Architecture:**

```go
// internal/core/broker/vhost/queue_pagecache.go
type PageCacheQueue struct {
    // Hot buffer for most recent messages
    hotBuffer   chan *Message  // Small, e.g., 1000 messages
    hotCapacity int
    
    // Memory-mapped segments for cold data
    segments    []*Segment
    mmapCache   *lru.Cache     // LRU cache of mmap'd segments
    
    // Read/write positions
    writeOffset uint64
    readOffset  uint64
}

func NewPageCacheQueue(name string, hotCapacity int) *PageCacheQueue {
    return &PageCacheQueue{
        hotBuffer:   make(chan *Message, hotCapacity),
        hotCapacity: hotCapacity,
        mmapCache:   lru.New(10), // Cache 10 mmap'd segments
    }
}

func (q *PageCacheQueue) Push(msg *Message) error {
    select {
    case q.hotBuffer <- msg:
        // Fast path: message fits in hot buffer
        return nil
    default:
        // Slow path: hot buffer full, write to segment
        return q.writeToSegment(msg)
    }
}

func (q *PageCacheQueue) Pop() (*Message, error) {
    select {
    case msg := <-q.hotBuffer:
        // Fast path: message in hot buffer
        return msg, nil
    default:
        // Slow path: read from memory-mapped segment
        return q.readFromSegment(q.readOffset)
    }
}

func (q *PageCacheQueue) readFromSegment(offset uint64) (*Message, error) {
    // Find segment containing offset
    segment := q.findSegment(offset)
    
    // Get or create memory mapping
    mmap, err := q.getMmap(segment)
    if err != nil {
        return nil, err
    }
    
    // Read from memory-mapped region (page cache)
    // OS handles caching automatically
    data := mmap.ReadAt(offset)
    return UnmarshalMessage(data)
}

func (q *PageCacheQueue) getMmap(segment *Segment) (*Mmap, error) {
    // Check LRU cache
    if cached, ok := q.mmapCache.Get(segment.Path()); ok {
        return cached.(*Mmap), nil
    }
    
    // Create new memory mapping
    mmap, err := NewMmap(segment.Path())
    if err != nil {
        return nil, err
    }
    
    // Add to cache
    q.mmapCache.Add(segment.Path(), mmap)
    
    return mmap, nil
}
```

**Memory Mapping Implementation:**

```go
// pkg/persistence/implementations/wal/mmap.go
package wal

import (
    "os"
    "syscall"
)

type Mmap struct {
    file   *os.File
    data   []byte
    length int
}

func NewMmap(path string) (*Mmap, error) {
    file, err := os.OpenFile(path, os.O_RDONLY, 0)
    if err != nil {
        return nil, err
    }
    
    stat, err := file.Stat()
    if err != nil {
        file.Close()
        return nil, err
    }
    
    // Memory map the file
    data, err := syscall.Mmap(
        int(file.Fd()),
        0,
        int(stat.Size()),
        syscall.PROT_READ,
        syscall.MAP_SHARED,
    )
    if err != nil {
        file.Close()
        return nil, err
    }
    
    return &Mmap{
        file:   file,
        data:   data,
        length: int(stat.Size()),
    }, nil
}

func (m *Mmap) ReadAt(offset int64) []byte {
    // Read from memory-mapped region
    // This is backed by OS page cache
    return m.data[offset:]
}

func (m *Mmap) Close() error {
    syscall.Munmap(m.data)
    return m.file.Close()
}
```

### Benefits

1. **Unbounded Queue Size**: Queues can hold millions of messages without OOM
2. **Automatic Memory Management**: OS handles eviction under pressure
3. **Warm Data Stays Hot**: Frequently accessed data stays in RAM
4. **Cold Data on Disk**: Infrequently accessed data on disk automatically
5. **Better Resource Utilization**: Memory shared with other processes

### AMQP Compatibility

✅ **Transparent caching** - no protocol changes needed

### Performance Characteristics

- **Hot data (in page cache)**: ~1-2µs read latency
- **Cold data (disk read)**: ~5-10ms read latency (SSD)
- **Memory usage**: ~1000 messages * avg size (hot buffer only)
- **Disk usage**: All messages

### Configuration

```go
type PageCacheConfig struct {
    HotBufferSize    int    // Number of messages in hot buffer
    MmapCacheSize    int    // Number of segments to keep mapped
    MadviseStrategy  string // "MADV_RANDOM", "MADV_SEQUENTIAL"
}
```

## Pattern 6: Segment Compaction

### Kafka's Approach

Background process removes consumed messages and keeps only necessary data based on retention policies.

### OtterMQ Implementation

```go
// pkg/persistence/implementations/wal/compaction.go
type CompactionManager struct {
    queue          *PageCacheQueue
    policy         RetentionPolicy
    compactionChan chan struct{}
    stopChan       chan struct{}
}

type RetentionPolicy struct {
    MinOffset      uint64        // Don't delete below this offset
    MaxAge         time.Duration // e.g., 7 days
    MaxSize        int64         // e.g., 100GB
    CompactOnClose bool          // Compact when queue is empty
}

func (cm *CompactionManager) Start() {
    go cm.compactionLoop()
}

func (cm *CompactionManager) compactionLoop() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            cm.compact()
        case <-cm.compactionChan:
            cm.compact()
        case <-cm.stopChan:
            return
        }
    }
}

func (cm *CompactionManager) compact() error {
    // Get minimum offset still needed by any consumer
    minOffset := cm.queue.GetMinConsumerOffset()
    
    // Find segments that can be deleted
    segmentsToDelete := cm.findDeletableSegments(minOffset)
    
    // Delete old segments
    for _, segment := range segmentsToDelete {
        if err := cm.deleteSegment(segment); err != nil {
            return err
        }
    }
    
    // Apply age-based retention
    cm.deleteSegmentsOlderThan(cm.policy.MaxAge)
    
    // Apply size-based retention
    cm.deleteSegmentsExceedingSize(cm.policy.MaxSize)
    
    return nil
}

func (cm *CompactionManager) findDeletableSegments(minOffset uint64) []*Segment {
    var deletable []*Segment
    
    for _, segment := range cm.queue.segments {
        // Can delete if all messages in segment are below minOffset
        if segment.MaxOffset() < minOffset {
            deletable = append(deletable, segment)
        }
    }
    
    return deletable
}

func (cm *CompactionManager) deleteSegment(segment *Segment) error {
    // Close memory mappings
    segment.Close()
    
    // Delete files
    os.Remove(segment.LogPath())
    os.Remove(segment.IndexPath())
    
    return nil
}
```

**Integration with Queue:**

```go
func (q *PageCacheQueue) StartCompaction(policy RetentionPolicy) {
    cm := &CompactionManager{
        queue:          q,
        policy:         policy,
        compactionChan: make(chan struct{}, 1),
        stopChan:       make(chan struct{}),
    }
    
    cm.Start()
    q.compactionManager = cm
}

func (q *PageCacheQueue) GetMinConsumerOffset() uint64 {
    return q.offsetTracker.GetMinimumOffset()
}
```

### Benefits

1. **Automatic Cleanup**: No manual purge needed
2. **Configurable Retention**: Time, size, or offset-based
3. **Disk Space Management**: Prevents unbounded growth
4. **Performance**: Runs in background, doesn't block operations

### AMQP Compatibility

✅ **Invisible to clients** - automatic background process

### Retention Strategies

**1. Offset-Based (Default)**
- Keep messages until all consumers ACK
- Natural for work queues

**2. Time-Based**
- Keep messages for N days
- Good for audit logs

**3. Size-Based**
- Keep last N GB of messages
- Prevents disk exhaustion

**4. Hybrid**
- Combination of above
- Most flexible

## Pattern 7: Pre-allocated Segments

### Kafka's Approach

Pre-allocate segment files to avoid filesystem fragmentation and metadata updates.

### OtterMQ Implementation

```go
// pkg/persistence/implementations/wal/segment.go
const (
    DefaultSegmentSize = 1 * 1024 * 1024 * 1024  // 1GB
)

func CreateSegment(path string, baseOffset uint64, size int64) (*Segment, error) {
    // Create file
    file, err := os.Create(path)
    if err != nil {
        return nil, err
    }
    
    // Pre-allocate space
    if err := file.Truncate(size); err != nil {
        file.Close()
        return nil, err
    }
    
    // On Linux, use fallocate for better performance
    if err := fallocate(file, size); err != nil {
        // Fall back to truncate (already done above)
    }
    
    return &Segment{
        path:       path,
        file:       file,
        baseOffset: baseOffset,
        size:       size,
        position:   0,
    }, nil
}

// Linux-specific optimization
func fallocate(file *os.File, size int64) error {
    return syscall.Fallocate(int(file.Fd()), 0, 0, size)
}

func (s *Segment) Append(data []byte) error {
    // Check if write would exceed pre-allocated size
    if s.position + int64(len(data)) > s.size {
        return ErrSegmentFull
    }
    
    // Write at current position (no file growth)
    _, err := s.file.WriteAt(data, s.position)
    if err != nil {
        return err
    }
    
    s.position += int64(len(data))
    return nil
}
```

**Segment Rotation:**

```go
func (q *WALQueue) rotateSegment() error {
    // Seal current segment (trim unused space)
    if err := q.activeSegment.Trim(); err != nil {
        return err
    }
    
    // Create new pre-allocated segment
    nextOffset := q.activeSegment.NextOffset()
    newSegment, err := CreateSegment(
        q.segmentPath(nextOffset),
        nextOffset,
        DefaultSegmentSize,
    )
    if err != nil {
        return err
    }
    
    q.activeSegment = newSegment
    q.segments = append(q.segments, newSegment)
    
    return nil
}

func (s *Segment) Trim() error {
    // Truncate to actual used size
    return s.file.Truncate(s.position)
}
```

### Benefits

1. **Predictable Performance**: No filesystem metadata updates during writes
2. **Reduced Fragmentation**: Contiguous allocation
3. **Better SSD Performance**: Aligned writes, reduced wear
4. **Faster Writes**: No file growth operations

### Trade-offs

- **Disk Space**: Pre-allocated space temporarily unused
- **Rotation**: Must rotate when segment full (not a problem with 1GB segments)

## Implementation Roadmap

### Phase 1: Foundation (Months 1-3)
**Goal:** Implement log-structured storage and offset tracking

**Tasks:**
1. Create `pkg/persistence/implementations/wal/` package structure
2. Implement `Segment` with append-only log
3. Implement `Index` for efficient offset lookup
4. Build `WALQueue` with segment rotation
5. Implement `OffsetTracker` for consumer offsets
6. Migrate one queue type to WAL for testing

**Deliverables:**
- Working WAL implementation
- Unit tests for segment operations
- Benchmark comparison vs current JSON approach
- Documentation for WAL format

**Success Metrics:**
- Write throughput > current implementation
- Recovery time < 5s for 1M messages
- Memory usage < 100MB for 10M message queue

### Phase 2: Performance Optimizations (Months 4-5)
**Goal:** Add batching, page cache, and zero-copy

**Tasks:**
1. Implement `BatchWriter` with configurable flush policy
2. Add memory-mapped segment reads
3. Implement page cache-based queue (`PageCacheQueue`)
4. Add zero-copy delivery for large messages
5. Implement LRU cache for mmap'd segments
6. Configuration options for flush modes

**Deliverables:**
- Batched write implementation
- Memory-mapped read path
- Zero-copy delivery (Linux)
- Performance benchmarks

**Success Metrics:**
- 5-10x write throughput improvement
- 2-3x CPU reduction for large messages
- Memory usage proportional to hot set, not queue size

### Phase 3: Lifecycle Management (Month 6)
**Goal:** Add compaction and retention policies

**Tasks:**
1. Implement `CompactionManager`
2. Add retention policies (offset, time, size)
3. Background compaction goroutine
4. Segment pre-allocation
5. Graceful shutdown and recovery
6. Metrics and monitoring

**Deliverables:**
- Automatic segment cleanup
- Configurable retention policies
- Pre-allocated segment support
- Monitoring integration

**Success Metrics:**
- Automatic disk space management
- No manual purge needed
- Configurable retention honored
- Background compaction < 5% CPU overhead

### Phase 4: Production Readiness (Month 7-8)
**Goal:** Testing, monitoring, and migration

**Tasks:**
1. Comprehensive test suite (unit, integration, e2e)
2. Load testing and benchmarking
3. Migration tool from JSON to WAL
4. Monitoring and metrics integration
5. Documentation and examples
6. Configuration management

**Deliverables:**
- Full test coverage
- Migration guide and tooling
- Performance benchmarks
- Production documentation

**Success Metrics:**
- Zero data loss in tests
- Successful migration path
- Clear performance improvements
- Production-ready documentation

## Configuration Examples

### Basic Configuration

```yaml
# config/persistence.yaml
persistence:
  type: "wal"  # "json" | "wal"
  
  wal:
    # Segment settings
    segment_size: 1073741824  # 1GB
    pre_allocate: true
    
    # Write settings
    flush_mode: "batch"       # "immediate" | "batch" | "async"
    batch_max_size: 100
    batch_max_wait: "10ms"
    
    # Retention policies
    retention:
      min_offset_based: true  # Keep until all consumers ACK
      max_age: "168h"         # 7 days
      max_size: 107374182400  # 100GB
      
    # Performance tuning
    page_cache:
      hot_buffer_size: 1000
      mmap_cache_size: 10
      
    zero_copy:
      enabled: true
      min_message_size: 65536  # 64KB
```

### Per-Queue Override

```yaml
# Queue-specific configuration
queues:
  high_priority:
    persistence:
      flush_mode: "immediate"  # Immediate durability
      
  logs:
    persistence:
      retention:
        max_age: "720h"  # 30 days for logs
        
  transient:
    persistence:
      flush_mode: "async"  # Maximum throughput
```

## Monitoring and Metrics

### Key Metrics to Track

```go
// pkg/persistence/implementations/wal/metrics.go
type WALMetrics struct {
    // Write metrics
    WritesTotal       int64
    WriteLatencyP50   time.Duration
    WriteLatencyP99   time.Duration
    BatchSize         float64
    
    // Read metrics
    ReadsTotal        int64
    ReadLatencyP50    time.Duration
    CacheHitRate      float64
    
    // Storage metrics
    ActiveSegments    int
    TotalDiskUsage    int64
    OldestOffset      uint64
    NewestOffset      uint64
    
    // Compaction metrics
    CompactionsTotal  int64
    SegmentsDeleted   int64
    SpaceReclaimed    int64
}
```

### Prometheus Integration

```go
// Export metrics for Prometheus
func (m *WALMetrics) Export() {
    prometheus.Register(prometheus.NewGaugeFunc(
        prometheus.GaugeOpts{
            Name: "ottermq_wal_active_segments",
            Help: "Number of active WAL segments",
        },
        func() float64 { return float64(m.ActiveSegments) },
    ))
    
    // ... more metrics
}
```

## Testing Strategy

### Unit Tests
- Segment append/read operations
- Index lookup correctness
- Offset tracking accuracy
- Batch writer timing
- Compaction logic

### Integration Tests
- WAL queue push/pop
- Consumer offset management
- Segment rotation
- Recovery from crashes
- Compaction with active consumers

### Performance Tests
- Write throughput benchmarks
- Read latency benchmarks
- Memory usage profiling
- Disk I/O patterns
- Zero-copy effectiveness

### E2E Tests
- AMQP protocol compliance
- Multi-consumer scenarios
- Transaction behavior
- Crash recovery
- Migration from JSON

## Migration Path

### Step 1: Parallel Implementation
Run both JSON and WAL side-by-side for testing

### Step 2: Gradual Rollout
Migrate queues one by one to WAL

### Step 3: Migration Tool
```bash
# Convert existing JSON queues to WAL format
ottermq-migrate --from json --to wal --queue myqueue
```

### Step 4: Validation
Verify data consistency and performance

### Step 5: Deprecation
Remove JSON implementation after successful migration

## References

- **Kafka Storage Internals**: https://kafka.apache.org/documentation/#design_filesystem
- **Log-Structured Storage**: https://engineering.linkedin.com/distributed-systems/log-what-every-software-engineer-should-know-about-real-time-datas-unifying
- **Zero-Copy Techniques**: https://developer.ibm.com/articles/j-zerocopy/
- **Memory-Mapped Files**: https://en.wikipedia.org/wiki/Memory-mapped_file
- **Segment Compaction**: https://kafka.apache.org/documentation/#compaction
