## Prerequisites

- CrowdStrike Integration configured in Aembit (see the CrowdStrike Integration module)
- CrowdStrike Falcon Sensor installed on the client workload

## Values Reference

No customer-specific values needed - all values are configured in the CrowdStrike Integration infrastructure module.

## Aembit Configuration

1. Navigate to **Access Conditions** and create a new **CrowdStrike Access Condition**
   - **Name**: descriptive label (e.g., `CrowdStrike Device Health`)
   - **Integration**: select the CrowdStrike Integration configured in the infrastructure module
   - **Condition**: Enable **Restrict Reduced Functionality Mode**
   - **Time**: Set **Last Seen** to 1 hour

## Verification

- Stop the CrowdStrike Falcon Agent on the client workload instance, wait for the Last Seen window to expire, then attempt to access a protected resource - confirm access is denied
- Restart the agent and confirm access resumes
- Navigate to **Activity** and confirm the access denial event is logged

## Troubleshooting

- **Access not blocked when agent is stopped:** The Access Condition is not attached to the Access Policy. Navigate to **Access Policies** and confirm the CrowdStrike Access Condition is linked to the relevant policy
- **Agent not recognized:** The Falcon Agent on the client workload may not be reporting to the same CrowdStrike tenant. Verify the agent is enrolled and visible in the CrowdStrike Falcon Console under **Hosts**
