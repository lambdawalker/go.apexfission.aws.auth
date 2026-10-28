# Implement Apexfission: Extensible Go AWS Application Framework

## Mission

Implement **Option C: A modular, extensible Go framework for AWS applications**.

**Target repository:**  
https://github.com/lambdawalker/go.apexfission.aws.auth

**Reference implementation:**  
https://github.com/lambdawalker/go.attestra.aws.auth

The objective is to transform the existing Attestra authentication architecture into a reusable, versioned Go framework that other applications can import and extend.

A future banking, sales, or identity application must be able to reuse authentication, administration, AWS infrastructure, environment management, and deployment capabilities without copying or forking the framework.

This is an implementation task, not just an architecture exercise.

## 1. Architectural principles

1. **Composition over inheritance.** Applications register modules that contribute functionality.
2. **Static Go composition.** Use standard Go modules, interfaces, and dependency management rather than dynamic executable plugins.
3. **Extensible by design.** Applications can add API endpoints, admin pages, AWS resources, configuration, deployment steps, and permissions.
4. **AWS-native.** Preserve AWS Lambda, API Gateway, Cognito, DynamoDB, S3, SQS, SES, IAM, and other appropriate services.
5. **Pulumi for infrastructure.** Continue using Pulumi with Go. Do not recreate Pulumi's provisioning and state management functionality using the AWS SDK.
6. **Backward compatibility.** Keep the existing Attestra implementation functional while extracting reusable functionality.
7. **Security by default.** Apply least-privilege IAM, explicit authorization, environment isolation, and secure handling of credentials.
8. **Cross-platform tooling.** Support Windows, Linux, and macOS using Go.
9. **Versioned distribution.** Consumers should install the framework using conventional Go module dependencies.
10. **Clear ownership.** The framework owns reusable infrastructure and services; consuming applications own their business-specific functionality.

## 2. Repository investigation

Before implementing, inspect both repositories thoroughly.

The existing Attestra repository contains:

- Authentication services and AWS Lambda handlers.
- Email verification and passkey workflows.
- Cognito identity integration.
- ID document capture functionality.
- An administration dashboard.
- Runtime configuration, revisions, and administrative grants.
- Environment registry and deployment coordination.
- Pulumi Go infrastructure projects in `infra` and `infra-index`.
- A Go deployment tool under `tools/deploy`.
- AWS authentication, credential storage, DNS, Cloudflare, GitHub Actions, and teardown workflows.

The deployment tool already contains reusable subsystems including `workflow`, `build`, `aws`, `dns`, `github`, `vault`, `registry`, and `ui`.

**Do not rewrite working subsystems without justification.**

Identify reusable components, application-specific behavior, coupling, existing contracts, and migration risks.

Inspect documentation, tests, and existing architecture decisions before changing behavior.

Treat Attestra as the reference implementation, not as a collection of files to copy blindly.

## 3. Build the framework

Implement the foundation in `go.apexfission.aws.auth`.

### A. Module system

Create a stable public Go API allowing applications to register modules.

Modules should be able to contribute:

- Runtime services.
- Public and authenticated API routes.
- Admin API endpoints.
- Admin dashboard pages and navigation.
- Configuration schemas and validators.
- AWS infrastructure components.
- Lambda build targets.
- IAM permission requirements.
- Deployment outputs and health checks.
- Optional environment lifecycle operations.

Prefer focused, optional interfaces rather than requiring every module to implement every extension method.

Support module identification, dependencies, version compatibility, duplicate registration detection, and deterministic initialization order.

Separate runtime registration from infrastructure provisioning so Lambda binaries do not inherit Pulumi dependencies.

### B. Authentication core

Extract reusable authentication functionality from Attestra.

Preserve existing semantics wherever possible, including email verification, passkeys, sign-in, sessions, and Cognito integration.

Design authentication and authorization as separate concerns.

Applications must be able to declare permissions and protect routes without modifying the framework's internals.

Document token validation, trust boundaries, claims, and how applications should integrate authenticated users.

Make ID capture an optional capability rather than an unavoidable dependency of every application.

### C. API extension system

Develop a declarative API registration mechanism.

Applications should be able to register:

- HTTP method and path.
- Handler or Lambda integration.
- Authentication requirements.
- Authorization policies.
- Request validation.
- Required infrastructure dependencies.

Prevent duplicate or conflicting routes.

Support explicit public endpoints, but make authenticated access the default for protected application functionality.

Separate API Gateway infrastructure registration from runtime request handling.

Consider OpenAPI generation if it can be implemented without excessive complexity.

### D. Extensible administration dashboard

Transform the existing admin experience into an application-extensible shell.

The framework should own:

- Administrative authentication.
- Authorization and permission checks.
- Navigation and layout.
- Environment selection.
- Configuration publication.
- Audit history.
- Shared UI conventions.

Applications should be able to register their own navigation sections, pages, frontend assets, configuration panels, and admin API endpoints.

Do not require every administration page to be implemented in Go. Support frontend assets produced by standard web build tools.

Provide a page manifest or comparable registration contract.

Maintain strong isolation between frontend presentation permissions and server-enforced authorization.

Extend configuration storage with namespaced module-specific schemas, validation, revisioning, and migration support.

### E. Reusable Pulumi infrastructure

Convert relevant infrastructure into reusable Go Pulumi components.

The framework should expose composable resources and outputs for authentication, administration, APIs, and other core services.

An application must be able to add its own DynamoDB tables, S3 buckets, Lambda functions, queues, IAM policies, and API Gateway routes.

Avoid hardcoded archive paths, resource inventories, and assumptions about the number of Lambda functions.

Use stable Pulumi resource names and carefully plan existing resource migrations.

Do not perform destructive resource replacements merely to simplify framework extraction.

Application infrastructure should remain owned by the application's Pulumi stack.

### F. Deployment engine

Extract and generalize the existing Go deployment wizard.

It should support operations comparable to:

```bash
attestra init
attestra build
attestra preview --env dev
attestra deploy --env dev
attestra status --env dev
attestra teardown --env dev
```

Names and flags may be refined if the existing code suggests a better design.

Preserve useful existing capabilities:

- AWS credentials and SSO.
- Environment selection.
- Encrypted credential vault.
- Pulumi S3 backend.
- Build orchestration.
- Parallel Lambda packaging.
- Cloudflare DNS configuration.
- SES identity setup.
- GitHub Actions and OIDC deployment.
- Deployment checkpoints and recovery.
- Environment registry coordination.
- Deployment validation.
- Controlled teardown.

Make build targets, infrastructure stacks, configuration, deployment steps, and outputs application-aware.

The deployment engine should orchestrate Pulumi, not replace it.

The same deployment behavior should work locally and in GitHub Actions.

### G. Application manifest

Provide a simple, versioned application configuration format.

It should identify the application, enabled modules, AWS deployment settings, and environments.

Use conventional Go dependencies as the authoritative source of executable module implementations and versions.

The manifest must not dynamically execute arbitrary code.

Provide validation with actionable error messages.

## 4. Reference application

Implement a small example application to prove extensibility.

A sales-oriented example is appropriate.

It should demonstrate:

- Importing the framework through Go modules.
- Enabling authentication.
- Registering application-specific API routes.
- Registering one new admin dashboard section.
- Creating an application-owned DynamoDB table.
- Assigning appropriate IAM permissions.
- Building its Lambda handlers.
- Integrating everything into the deployment configuration.

The example should contain actual working components and tests, not just illustrative pseudocode.

The essential test is that the application adds functionality **without modifying framework internals**.

## 5. Suggested package boundaries

Investigate a structure along these lines:

```text
go.apexfission.aws.auth/
├── core/
├── auth/
├── authorization/
├── api/
├── admin/
├── config/
├── runtime/
├── modules/
├── aws/
├── pulumi/
├── deploy/
├── cmd/
├── examples/
├── docs/
└── tests/
```

This is conceptual, not a mandatory directory structure.

Keep package boundaries focused, avoid circular dependencies, and minimize unnecessary AWS SDK or Pulumi dependencies in runtime packages.

Do not immediately create numerous additional GitHub repositories. Establish stable interfaces first and split modules only when justified.

## 6. Security and lifecycle requirements

Security is especially important because this framework will eventually support applications handling sensitive information.

Implement or document safeguards for:

- Least-privilege IAM.
- JWT signature, issuer, audience, expiration, and token-use validation.
- Authentication versus authorization.
- Namespaced application permissions.
- Environment isolation.
- Secret handling and encryption.
- Administrative audit trails.
- Safe infrastructure previews.
- Protected resources.
- Explicit destructive-operation permissions.
- Application-specific data deletion policies.
- Secure default configurations.

The environment-reset mechanism must not automatically obtain authority over every resource contributed by an application.

Production teardown and destructive operations must require explicit safeguards.

Do not run destructive actions against existing Attestra environments.

## 7. Migration and compatibility

Create a migration strategy for the original repository:

https://github.com/lambdawalker/go.attestra.aws.auth

The objective is eventually to make Attestra consume the Apexfission framework.

Avoid a big-bang rewrite.

Provide a compatibility matrix covering APIs, runtime behavior, Cognito resources, DynamoDB tables, admin configuration, deployment state, and GitHub Actions.

Where existing Pulumi resources need to move into reusable components, document and test the import or alias strategy to prevent unintended replacement.

Do not modify or deploy the original repository without establishing a safe migration plan.

## 8. Implementation workflow

Work in a feature branch of `go.apexfission.aws.auth`.

Proceed through logical milestones:

**Milestone 1: Architecture and contracts**
- Audit both repositories.
- Create architecture documentation and module interfaces.
- Establish the new Go module and test infrastructure.

**Milestone 2: Reusable authentication runtime**
- Extract reusable authentication packages.
- Implement route registration and authorization integration.
- Add unit and integration tests.

**Milestone 3: Infrastructure composition**
- Introduce reusable Pulumi components.
- Implement extension registration.
- Validate deployment previews without making live AWS changes.

**Milestone 4: Admin extensibility**
- Implement the admin shell's extension mechanism.
- Demonstrate application-owned pages and endpoints.
- Preserve authentication, permissions, and configuration history.

**Milestone 5: Generalized deployment tooling**
- Refactor the existing Go deployment logic into reusable components.
- Make Lambda builds and infrastructure definitions discoverable.
- Support application-specific deployment configurations.

**Milestone 6: Reference application**
- Implement the example sales application.
- Verify that its infrastructure, API and admin features compose with the framework.

**Milestone 7: Documentation and verification**
- Document installation, module creation, deployment, extension APIs, security, and migration.
- Run all available tests and static checks.
- Create a pull request with the results.

Implement working functionality incrementally. Do not stop after producing architecture documentation.

## 9. Completion criteria

The work is successful when:

1. The framework is importable as a Go module.
2. A consuming application can register its own module without editing framework code.
3. Application-specific APIs integrate with the authentication and authorization infrastructure.
4. Application-specific administrative pages appear in the shared dashboard.
5. Application-specific AWS resources compose with reusable Pulumi components.
6. The deployment tool can build and preview a consuming application.
7. Existing Attestra behavior has a documented compatibility and migration path.
8. Automated tests validate the core contracts and a complete reference application.
9. Installation and integration are clearly documented.
10. A pull request contains the implementation, test results, architectural decisions, and known limitations.

Do not claim that deployment was verified if only local compilation or Pulumi preview was performed.

## 10. Expected deliverables

Produce:

- Working framework source code.
- Public Go extension interfaces.
- Reusable authentication services.
- Extensible API and admin infrastructure.
- Reusable Pulumi components.
- Generalized deployment tooling.
- Working reference application.
- Unit and integration tests.
- Architecture and integration documentation.
- Migration guide for Attestra.
- Pull request for review.

Preserve existing security and deployment behavior whenever possible.

**Begin by inspecting the two repositories and creating a concise implementation architecture. Then proceed to implementation in the target repository, validating each milestone. Do not stop after the initial assessment.**

The ultimate developer experience should be straightforward: create a Go application, import Apexfission, register application modules, and deploy a complete AWS-backed application with authentication and an extensible administration console.