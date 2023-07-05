# ingester

The ingester is responsible for collecting the data from the distributors and storing this data to disk. This module has
a few components:

- **WAL** - This is the Write Ahead Log that tries to ensure zero data loss
- **tenantBlockManager** - This is responsible for managing the data for each tenant
- **search** - This is an optional route to search live snapshots (snapshots that have been ingested but not yet flushed
  to storage)

# Overview

The ingester accepts data from the distributors via the '[IngesterService](../../pkg/deeppb/deep.proto)' service. This
data is then passed to an instance of the 'tenantBlockManager'. Which will store the data in memory, once in memory it
is periodically flushed to the WAL (also on shutdown). These WAL blocks are then periodically flushed to a local
'backend block' which is a local version of how it is stored in the backend (s3). These local blocks are then
periodically flushed to the backend storage (s3).

On start up we have to replay any data that has been written to the local disk. This ensures that any data that has been
accepted, but not flushed to storage is not lost during a crash.

## IngesterService

This is the main entry for data, it is defined in [ingester.go](./ingester.go). This will simply find or create a new
tenantBlockManager and pass the data to it for processing.

## tenantBlockManager

This deals with reading and writing data to the various forms of blocks that are used in the ingester:

- **liveSnapshot** - An in memory 'block' that has not been written to disk yet
- **WAL Block** - A collection of smaller blocks, each being the contents of liveSnapshots
- **local backend Block** - This is the compacted WAL block that created prior to flushing to storage
- **backend block** - The block that is a copy of the **local backend block** but is sent to storage

Each stage is written to and flushed to the next using periodic checks.

### ingest to liveSnapshots

This is the first step of ingest, as soon as the data is accepted from the IngesterService, it is written to the
tenantBlockManager into the liveSnapshots.

### liveSnapshots to WAL Block

This is the main way to prevent data loss on crash or restart, periodically (using the FlushCheckPeriod config value,
default 10 seconds) the data form the liveSnapshots is written to a new file in the current WAL block.

### WAL Block to local backend Block

Every time WAL Block is updated it is checked if it is ready, determined by its age and size. Once it is ready it is
queued to as ready. Then a new WAL block will be started for the next set of liveSnapshots. An operation of '
opKindComplete' is then enqueued so the completed block can be handled.

The operations are continuously checked, and as soon as the processors have capacity they will process the completed
block. This entails compacting and writing the data into a backend block and storing it locally, becoming a 'local
backend block'. Once complete another operation is enqueued 'opKindFlush'.

### local backend Block to backend Block

When the operation 'opKindFlush' is processed, then tenantBlockManager will write the local block to the backend
storage. The local block is then marked as flushed, and will be deleted once the 'CompleteBlockTimeout' has expired.

