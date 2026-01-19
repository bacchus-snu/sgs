# Request a workspace

[id]: https://id.snucse.org
[sgs-request]: https://sgs.snucse.org/request

> Before continuing, you must already have a [SNUCSE ID][id] account registered
> in either the `undergraduate` or `graduate` groups.

To request a workspace, fill out the Workspace request form on the SGS
[workspace management page][sgs-request].

![Workspace request form](images/request-workspace/ws-request.png)

After submitting the form, your workspace will be in the "Pending approval"
state.

![Pending approval](images/request-workspace/ws-pending.png)

Your workspace request will be reviewed by Bacchus volunteers. Once approved,
your workspace will transition to the "Enabled" state.

![Enabled](images/request-workspace/ws-enabled.png)

You may request updates to your quota or the users list at any time. Use the
"reason" field to explain the purpose of the change. Similar to workspace
requests, change requests will be reviewed by Bacchus volunteers.

![Request changes](images/request-workspace/ws-changes.png)

## GPU Resource Allocation Request Guidelines

**Important:** In order to ensure that our limited GPU resources are allocated in an optimal and transparent manner, we require all researchers to provide a clear and concise justification for their resource requests.

When submitting a request for GPU resources, please provide a concise explanation of how the allocated resources will be utilized to support your research objectives. This justification should include:

- If you are requesting a workspace in `graduate` Nodegroup, it is mandatory to provide the name of your advisor and/or the name of the principal investigator (PI) of the project.
- A brief overview of your research project, including its goals and objectives.
- If you (or your team) are targeting a specific journal or conference, please include the CfP(Call for Papers) URL.
  - This may help us to prioritize your resource request accordingly if the deadlines are nearer.
- A description of the specific GPU resources required, including:
  - A brief description of workloads, you are going to run on the GPU(s).
  - If you are requesting multiple GPUs, please provide detailed justification with regard to VRAM requirements or research challenges.
- A description of your storage requirements. Explain why you need a specific amount of storage by providing the details on:
  - The size of the model (i.e. the approximate number of model parameters)
  - The size and number of datasets.
  - Any specific storage-related constraints or limitations.
- If you entered non-zero values to 'CPU Guaranteed' or 'Memory Guaranteed', please provide a detailed justification for your request.
  - In most cases, you don't need to obtain guaranteed CPU or Memory resources, and guaranteed resources only increase the risk of your workloads being terminated due to underutilized guaranteed resources.
- An estimate of the expected duration of the project.

<div class="warning">
Failure to provide a satisfactory justification may result in delays or rejection of your resource request.
</div>

<div class="warning">
For collaborative projects involving multiple researchers, please submit a single workspace request and subsequently add your team members to the workspace by providing their id.snucse.org account usernames. This approach eliminates the need for individual workspace requests for each researcher.
</div>

Applications may be submitted in either Korean or English, and our reviewers will be able to assess them in either language.

### Why this is important?

As system administrators, we are accountable to the CSE department for ensuring that our GPU resources are allocated in a way that maximizes their impact on research projects. By providing a clear justification for your resource requests, you are helping us to:

- Evaluate the merits and urgency of each request and prioritize allocations accordingly.
- Report to the CSE department on the effective utilization of our GPU resources and the impact on research outcomes.
- Continuously improve our resource allocation processes to better support the research community.

Thank you for your understanding, and please feel free to reach out to us at [contact@bacchus.snucse.org](mailto:contact@bacchus.snucse.org).
