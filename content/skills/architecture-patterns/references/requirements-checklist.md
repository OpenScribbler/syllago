# Requirements Checklist

Use this checklist during requirements gathering to ensure complete understanding before design.

## Functional Requirements

- [ ] What problem does this solve?
- [ ] Who are the users/consumers?
- [ ] What are the core use cases?
- [ ] What integrations are needed?
- [ ] What are the inputs and outputs?
- [ ] What data needs to be stored?

## Non-Functional Requirements

### Scale
- [ ] Expected concurrent users?
- [ ] Requests per second (average/peak)?
- [ ] Data volume (initial/growth rate)?
- [ ] Geographic distribution?

### Availability
- [ ] Required uptime (99.9%, 99.99%)?
- [ ] Acceptable maintenance windows?
- [ ] Disaster recovery requirements?
- [ ] RTO/RPO targets?

### Latency
- [ ] Acceptable response times (p50, p99)?
- [ ] Real-time requirements?
- [ ] Batch processing acceptable?

### Security
- [ ] Data sensitivity classification?
- [ ] Compliance requirements (CMMC, SOC2, HIPAA)?
- [ ] Authentication/authorization requirements?
- [ ] Audit logging requirements?

### Cost
- [ ] Budget constraints?
- [ ] Cost optimization priority?
- [ ] Build vs buy preference?

## Constraints

### Technology
- [ ] Required technologies (language, platform)?
- [ ] Prohibited technologies?
- [ ] Existing systems to integrate with?

### Timeline
- [ ] Delivery deadline?
- [ ] Phased rollout acceptable?
- [ ] MVP vs full feature set?

### Team
- [ ] Available expertise?
- [ ] Team size and availability?
- [ ] Training requirements?

### Operations
- [ ] Deployment environment (cloud, on-prem, hybrid)?
- [ ] Existing monitoring/alerting infrastructure?
- [ ] On-call requirements?
- [ ] Deployment frequency expectations?
