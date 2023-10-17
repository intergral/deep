# Unhealthy Ring Member

## Meaning

This error happens when a member of the ring has become unhealthy. This could be due to an unclean shutdown or crash.

## Impact

If the node remains unhealthy in the ring it is possible to get inaccurate response for queries.

## Diagnosis

Check the logs for the ring members.

Check the ring member endpoints to check the state of the rings:

- /tracepoint/ring
- /ingester/ring
- /compactor/ring
- /metrics-generator/ring
- /distributor/ring

These can be narrowed down to the failing ring by using the name label in the alert.

## Mitigation

There is no generic way to correct the failure, it would depend on the reason for the failure. There are a few options.

### Forget node

If the ring is otherwise healthy then you can simply 'Forget' the node by using the action on the appropriate state page
listed above. This will remove the node from the ring and redistribute the tokens.
