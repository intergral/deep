# Missing Ring Node

## Meaning

This error happens when a ring is configured to have _n_ nodes but the actual number of nodes in the ring is either more
or less than that number.

## Impact

If the number of nodes continues to be in error, then the health of the ring can become unstable.

## Diagnosis

The diagnosis of the alert depends on if there are more or fewer nodes than expected.

### More nodes

If there are more nodes than there should be this could be due to a failure to shut down a node correctly. This can lead
to an [unhealthy node](./unhealthy_ring_node.md) scenario.

It could also be a sign that the number of replicas has been changed manually to address a scaling issue. Additionally,
it is possible that the `HorizontalPodAutoscaler` as kicked in to address a resource issue. In either case of scaling
the helm chart config should be updated to reflect the changes if they are to become permanent. This way the alert
config will be updated to reflect this change.

### Fewer nodes

If there are fewer nodes than there should be this could be due to a failure to start a new node. This could be due to a
resource starvation on the kubernetes cluster. The description of the deployment should be reviewed, and any errors here
addressed. Reviewing the pod logs for any failures in start up is also advised.

## Mitigation

There is no generic way to correct this issue, it would depend on the cause. Using the notes above identify the error
and look for ways to resolve the root cause

{!_sections/bug_report.md!}.
