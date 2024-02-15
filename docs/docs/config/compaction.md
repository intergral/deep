# Compaction/Retention

Compaction and Retention are the methods that are used by Deep to reduce the number of blocks that are stored to both
improve performance by reducing the block count, and remove older data that is no longer needed.

## Compaction

Compaction works by grouping blocks by time frame and combining the blocks together to reduce the overall number of
blocks that have to be scanned when performing a query.

The compaction can be configured using the settings:

| Name                        | Default | Description                                                                |
|-----------------------------|---------|----------------------------------------------------------------------------|
| `compaction_window`         | 1h      | This is the maximum time range a block should contain.                     |
| `max_compaction_objects`    | 6000000 | This is the maximum number of Snapshots that will be stored in each block. |
| `max_block_bytes`           | 100 GiB | This is the maximum size in bytes that each block can be.                  |
| `block_retention`           | 14d     | This is the total time a block will be stored for.                         |
| `compacted_block_retention` | 1h      | This is the duration a compacted block will be stored for.                 |
| `compaction_cycle`          | 1h      | The time between each compaction cycle.                                    |

By modifying these settings you can control how often blocks are compacted, how big they should be and how much time
they should span. There is no one size fits all config for compaction.

# Retention

Retention is when blocks are deleted, once a block has been compacted it is marked for deletion. This deletion cycle
occurs based on the config and will scan for marked and eligible blocks to be deleted. A block is eligible for deletion
if it has been compacted and the `compacted_block_retention` period has expired.
