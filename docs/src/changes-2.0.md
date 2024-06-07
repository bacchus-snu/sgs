# Changes from 2.0

If you have used the SGS 2.0 service, you may notice some changes in the 3.0
service.

## Workspace request

We have developed a unified workspace and quota and management system. You can
now request a workspace directly from the web interface. You no longer need to
create UserBootstrap objects to create resource quotas.

## Nodes

We now manage multiple nodes in a single cluster. Each node belongs to a
nodegroup (`undergraduate` or `graduate`), and each workspace is assigned to a
nodegroup. Pods in your workspace are automatically modified to only run in
nodes in your nodegroup. If you need to run on a specific node, use
nodeSelectors in your pod configuration.

## Resource model

We no longer grant guaranteed CPU or memory (request) quotas by default. If you
are absolutely sure you need request quotas for your use-case, you must justify
your request in the workspace request form.

Pod CPU and memory limits are now automatically set to your workspace quota
value using LimitRanges. If you need to run multiple containers (multiple
containers in a pod, multiple pods, or even both), adjust the limits in your pod
configuration.

## Permissions

Users can now query node details. You no longer need to contact Bacchus to check
the status of available node resources.

Multiple users can now be added to a single workspace. If you are collaborating
with multiple users, for example for coursework, you can now share a single
workspace.

We now enforce the `baseline` Pod Security Standard. Contact us if this is too
restrictive for your use-case.

## Registry

Harbor projects are now created automatically upon workspace approval. You no
longer need to create the project manually.

We now automatically configure imagePullSecrets for your workspace under the
default ServiceAccount. You no longer need to configure this manually, or
specify `imagePullSecrets` in your pod configuration.
