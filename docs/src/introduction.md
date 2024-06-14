# SNUCSE GPU Service 3.0

SNUCSE GPU Service (SGS) 3.0 is a free service provided to students in the SNU
CSE department. Before using this service, make sure you read this manual very
carefully.

This service is provided for **research purposes only**. Please limit your use
to the reasons stated in your workspace application form.

## Links

- Workspace management page: [`sgs.snucse.org`][sgs]
- Registry: [`sgs-registry.snucse.org`][sgs-registry]
- Dashboard [`sgs-dashboard.snucse.org`][sgs-dashboard]

[sgs]: https://sgs.snucse.org
[sgs-registry]: https://sgs-registry.snucse.org
[sgs-dashboard]: https://sgs-dashboard.snucse.org

## Best-Effort

This service is provided on a best-effort basis. Bacchus volunteers will try our
best to ensure smooth operations for everyone, but service may be interrupted
either due to technical issues, maintenance, or because the resources are in use
by another user.

Your containers may be terminated, without warning, at any time. To prevent data
loss, ensure all important data is stored in persistent volumes. We cannot
recover lost data from terminated containers.

## Resource policy

There are two types of quotas: guaranteed (including GPU and storage) and
limits.

Guaranteed resources, when assigned to a running pod, are reserved exclusively
for the container for the duration of its run-time. This means that even if your
container is completely idle, other users will not be able to use those
resources.

If we discover over-allocation of guaranteed resources (including GPUs), without
actually using the resources, we may take appropriate action, including:

- Terminating the offending containers
- Disabling your workspace
- Restricting access to this service permanently

Limits are the maximum amount of resources that can be used across all
containers in the workspace. These may be overcommited, and we cannot guarantee
that compute resources will be available to you unless you use guaranteed
quotas.

## Cleanup

Bacchus may periodically clean up old workspaces to free up resources.
Specifically, we plan on performing cleanup operations at the following times:

- At the beginning of the spring/fall semesters
- At the beginning of the summer/winter breaks

We will contact users before performing any cleanup operations. If you do not
respond in time, your workspace will be first disabled and later deleted. We
cannot recover lost data once your workspace has been deleted.

## Security policy

As a general rule, each workspace has similar security properties to running on
a shared Unix machine with other users.

For example, other users may be able to see the following:

- names of your pods and containers
- images being run in your containers
- command lines of processes running in your containers

However, the following information is hidden from other users:

- the contents of your persistent volumes or ephemeral storage
- contents of your secrets
- your container logs

If you intend to test the security of the service, please inform us in advance.

## Contact

SGS is developed and maintained by volunteers at
[Bacchus](https://bacchus.snucse.org). If you have any questions, please reach
out to us at [contact@bacchus.snucse.org](mailto:contact@bacchus.snucse.org).
