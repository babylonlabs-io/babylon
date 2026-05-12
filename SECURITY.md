# Security Policy

## Introduction

Security researchers are essential in identifying vulnerabilities that may impact the Babylon ecosystem.
If you have discovered a security vulnerability in the Babylon chain or any repository managed by Babylon,
we encourage you to notify us using one of the methods outlined below.

### Guidelines for Responsible Vulnerability Testing and Reporting

1. **Refrain from testing vulnerabilities on our publicly accessible environments**, including but not limited to:
  - Babylon mainnet.
  - Babylon frontend.
  - Babylon public testnets.
  - Babylon testnet frontend.
2. **Avoid reporting security vulnerabilities through public channels, including GitHub issues.**

## Reporting Security Issues

To privately report a security vulnerability, please choose one of the following options:

### 1. Email

Send your detailed vulnerability report to `security@babylonlabs.io`.

### 2. GitHub Private Vulnerability Reporting

Utilize [GitHub's Private Vulnerability Reporting](https://github.com/babylonlabs-io/babylon/security/advisories/new)
for confidential disclosure.

## Submit Vulnerability Report

When reporting a vulnerability through either method, please include the following details to aid in our assessment:

- Type of vulnerability.
- Description of the vulnerability.
- Steps to reproduce the issue.
- Impact of the issue.
- Explanation of how an attacker could exploit it.

## Vulnerability Disclosure Process

1. **Initial Report**: Submit the vulnerability via one of the above channels.
2. **Confirmation**: We will confirm receipt of your report within 48 hours.
3. **Assessment**: Our security team will evaluate the vulnerability and inform you of its severity and the estimated
   time frame for resolution.
4. **Resolution**: Once fixed, you will be contacted to verify the solution.
5. **Public Disclosure**: Details of the vulnerability may be publicly disclosed after ensuring it poses no further risk.

During the vulnerability disclosure process, we ask security researchers to keep vulnerabilities and communications
around vulnerability submissions private and confidential until a patch is developed. Should a security issue require
a network upgrade, additional time may be needed to raise a governance proposal and complete the upgrade.

During this time:

- Avoid exploiting any vulnerabilities you discover.
- Demonstrate good faith by not disrupting or degrading Babylon's services.

## Severity Characterization

| Severity     | Description                                                              |
|--------------|--------------------------------------------------------------------------|
| **CRITICAL** | Immediate threat to critical systems (e.g., chain halts, funds at risk). |
| **HIGH**     | Significant impact on major functionality.                               |
| **MEDIUM**   | Impacts minor features or exposes non-sensitive data.                    |
| **LOW**      | Minimal impact.                                                          |

## Bug Bounty

Babylon Labs maintains an official bug bounty program on Immunefi:

https://immunefi.com/bug-bounty/babylon-labs/

Security researchers are encouraged to submit vulnerabilities through Immunefi whenever possible.

However, we recognize that some legitimate researchers may be unable to access Immunefi for administrative or account-related reasons. To ensure critical vulnerabilities can always be reported responsibly, Babylon Labs also accepts private submissions through the following alternative channels:

- Email: security@babylonlabs.io
- GitHub Private Vulnerability Reporting

### Reward Eligibility

Vulnerabilities that are eligible for bounty rewards will be considered for compensation regardless of whether they are submitted through:

- Immunefi
- Email
- GitHub Private Vulnerability Reporting

Reward decisions and amounts are determined based on the severity, impact, quality of the report, and adherence to responsible disclosure practices.

### Preferred Submission Method

Immunefi remains the preferred submission platform when available, but using an alternative private reporting channel will not reduce or eliminate bounty eligibility.

> [!WARNING]
> Targeting our production environments will disqualify you from receiving any bounty.

## Feedback on this Policy

For recommendations on how to improve this policy, either submit a pull request or email `security@babylonlabs.io`.
