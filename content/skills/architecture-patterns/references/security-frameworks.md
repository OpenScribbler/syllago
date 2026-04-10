# Security Frameworks Reference

Mapping architecture decisions to security compliance frameworks.

## Framework Overview

| Framework | Focus | When to Use |
|-----------|-------|-------------|
| **CMMC** | DoD contractor security | Defense contracts, CUI handling |
| **NIST 800-53** | Federal information systems | Government systems, comprehensive security |
| **OWASP Top 10** | Application security | Web/API application design |
| **MITRE ATT&CK** | Adversary tactics | Threat modeling, detection design |

## CMMC 2.0

Cybersecurity Maturity Model Certification for DoD contractors.

### Levels

| Level | Name | Controls | Assessment | Use Case |
|-------|------|----------|------------|----------|
| **Level 1** | Foundational | 17 practices | Annual self-assessment | FCI only |
| **Level 2** | Advanced | 110 controls (NIST 800-171) | Self or C3PAO | CUI handling |
| **Level 3** | Expert | 110+ (adds NIST 800-172) | Government-led | Critical CUI |

### 14 Security Domains

| Domain | Abbr | Key Focus |
|--------|------|-----------|
| Access Control | AC | Who can access what |
| Awareness & Training | AT | Security training |
| Audit & Accountability | AU | Logging and monitoring |
| Configuration Management | CM | Secure configurations |
| Identification & Authentication | IA | Identity verification |
| Incident Response | IR | Breach handling |
| Maintenance | MA | System maintenance |
| Media Protection | MP | Data storage protection |
| Personnel Security | PS | Employee security |
| Physical Protection | PE | Physical access |
| Risk Assessment | RA | Risk identification |
| Security Assessment | CA | Control validation |
| System & Comms Protection | SC | Network security |
| System & Info Integrity | SI | Malware, patching |

### Documenting CMMC in ADRs

Add a CMMC Mapping table: `| Domain | Control | How Addressed |` with entries like `AC | AC.L2-3.1.1 | [implementation detail]`.

## NIST 800-53 Rev 5

Comprehensive security and privacy controls for federal systems.

### 20 Control Families

| ID | Family | Description |
|----|--------|-------------|
| AC | Access Control | Limit system access |
| AT | Awareness and Training | Security awareness |
| AU | Audit and Accountability | Audit records, monitoring |
| CA | Assessment, Authorization, Monitoring | Control assessment |
| CM | Configuration Management | Baseline configs |
| CP | Contingency Planning | Disaster recovery |
| IA | Identification and Authentication | Identity proofing |
| IR | Incident Response | Incident handling |
| MA | Maintenance | System maintenance |
| MP | Media Protection | Media handling |
| PE | Physical and Environmental | Physical security |
| PL | Planning | Security planning |
| PM | Program Management | Security program |
| PS | Personnel Security | Personnel screening |
| PT | PII Processing and Transparency | Privacy controls |
| RA | Risk Assessment | Risk analysis |
| SA | System and Services Acquisition | Secure acquisition |
| SC | System and Communications Protection | Network security |
| SI | System and Information Integrity | Flaw remediation |
| SR | Supply Chain Risk Management | Supply chain security |

### Control Baselines

| Baseline | Controls | When to Use |
|----------|----------|-------------|
| Low | 149 | Minimal security needs |
| Moderate | 287 | Most federal systems |
| High | 370 | Critical/sensitive systems |

### Documenting NIST 800-53 in Designs

Add a controls table: `| Control | Requirement | Implementation |` with entries like `AC-2 | Account Management | [detail]`.

## OWASP Top 10:2025

| Rank | Category | Architecture Considerations |
|------|----------|----------------------------|
| A01 | Broken Access Control | Authorization design, RBAC/ABAC |
| A02 | Security Misconfiguration | Secure defaults, hardening |
| A03 | Software Supply Chain | Dependency management, SBOM |
| A04 | Cryptographic Failures | Key management, encryption design |
| A05 | Injection | Input validation, parameterization |
| A06 | Insecure Design | Threat modeling, secure patterns |
| A07 | Authentication Failures | Auth flow design, MFA |
| A08 | Data Integrity Failures | Signing, verification |
| A09 | Security Logging Failures | Audit logging design |
| A10 | Mishandling Exceptions | Error handling, fail-safe |

Map designs to OWASP with: `| Risk | Mitigation in This Design |` table.

## MITRE ATT&CK

Adversary tactics, techniques, and procedures for threat modeling.

### Enterprise Tactics (Attack Phases)

| Tactic | ID | Description | Design Consideration |
|--------|-----|-------------|---------------------|
| Reconnaissance | TA0043 | Info gathering | Minimize exposed metadata |
| Resource Development | TA0042 | Attacker prep | N/A (pre-attack) |
| Initial Access | TA0001 | Entry point | Reduce attack surface |
| Execution | TA0002 | Running code | Sandboxing, least privilege |
| Persistence | TA0003 | Maintaining access | Integrity monitoring |
| Privilege Escalation | TA0004 | Gaining access | Role separation |
| Defense Evasion | TA0005 | Avoiding detection | Comprehensive logging |
| Credential Access | TA0006 | Stealing creds | Credential protection |
| Discovery | TA0007 | Learning environment | Network segmentation |
| Lateral Movement | TA0008 | Moving through network | Zero trust, microsegmentation |
| Collection | TA0009 | Gathering data | Data classification |
| Command and Control | TA0011 | Attacker communication | Egress filtering |
| Exfiltration | TA0010 | Stealing data | DLP, monitoring |
| Impact | TA0040 | Damage | Backups, resilience |

Map threat models to ATT&CK with: `| Attack Scenario | Tactics | Mitigations |` table.
