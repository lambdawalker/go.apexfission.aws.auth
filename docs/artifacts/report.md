# Artifact report

Outcome: ****

22 main artifacts · 74 children · 0 present · 0 absent · 0 recorded · 96 unknown · 96 planned changes

Cached evidence loaded: false

- Application: 
- Repository: go.apexfission.aws.auth
- Environment: preview
- Account: 
- Region: 
- Backend: 
- Project: 
- Stack: 
- Revision: 
- Operation: 
- Started: not checked
- Finished: not checked
- Duration: incomplete

Warning: illustrative\_offline\_catalog

## email-sender-dkim

- Logical identity: urn:pulumi:preview::offline-preview::aws:ses/domainDkim:DomainDkim::email-sender-dkim
- Type: aws:ses/domainDkim:DomainDkim
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## email-sender

- Logical identity: urn:pulumi:preview::offline-preview::aws:ses/domainIdentity:DomainIdentity::email-sender
- Type: aws:ses/domainIdentity:DomainIdentity
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## attesta-users

- Logical identity: urn:pulumi:preview::offline-preview::aws:cognito/userPool:UserPool::attesta-users
- Type: aws:cognito/userPool:UserPool
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### public-auth-client

- Logical identity: urn:pulumi:preview::offline-preview::aws:cognito/userPoolClient:UserPoolClient::public-auth-client
- Type: aws:cognito/userPoolClient:UserPoolClient
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-runtime-configuration

- Logical identity: urn:pulumi:preview::offline-preview::aws:dynamodb/table:Table::auth-runtime-configuration
- Type: aws:dynamodb/table:Table
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## email-proofs

- Logical identity: urn:pulumi:preview::offline-preview::aws:dynamodb/table:Table::email-proofs
- Type: aws:dynamodb/table:Table
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## environment-reset-control

- Logical identity: urn:pulumi:preview::offline-preview::aws:dynamodb/table:Table::environment-reset-control
- Type: aws:dynamodb/table:Table
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## app-config-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::app-config-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### app-config-data

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicy:RolePolicy::app-config-data
- Type: aws:iam/rolePolicy:RolePolicy
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### app-config

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::app-config
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### app-config-invoke

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::app-config-invoke
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### app-config-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::app-config-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-email-complete-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-email-complete-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-complete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-email-complete
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-email-complete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-email-complete
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-complete-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-email-complete-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-email-start-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-email-start-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-start

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-email-start
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-email-start

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-email-start
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-start-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-email-start-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-passkey-complete-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-passkey-complete-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-complete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-passkey-complete
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-passkey-complete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-passkey-complete
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-complete-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-passkey-complete-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-passkey-start-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-passkey-start-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-start

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-passkey-start
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-passkey-start

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-passkey-start
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-start-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-passkey-start-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-password-complete-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-password-complete-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-complete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-password-complete
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-password-complete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-password-complete
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-complete-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-password-complete-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-password-set-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-password-set-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-set-enrollment

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicy:RolePolicy::auth-password-set-enrollment
- Type: aws:iam/rolePolicy:RolePolicy
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-set

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-password-set
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-password-set

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-password-set
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-set-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-password-set-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-password-start-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-password-start-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-start

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-password-start
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-password-start

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-password-start
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-start-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-password-start-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-refresh-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-refresh-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-refresh

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-refresh
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-refresh

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-refresh
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-refresh-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-refresh-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## auth-status-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::auth-status-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-status-enrollment

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicy:RolePolicy::auth-status-enrollment
- Type: aws:iam/rolePolicy:RolePolicy
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-status

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::auth-status
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-auth-status

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-auth-status
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-status-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::auth-status-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## challenge-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::challenge-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### challenge-permissions

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicy:RolePolicy::challenge-permissions
- Type: aws:iam/rolePolicy:RolePolicy
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### cognito-grant-challenge

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::cognito-grant-challenge
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-cognito-challenge

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-cognito-challenge
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### challenge-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::challenge-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## environment-reset-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::environment-reset-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### environment-reset-data

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicy:RolePolicy::environment-reset-data
- Type: aws:iam/rolePolicy:RolePolicy
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### environment-reset

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::environment-reset
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### environment-reset-schedule

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::environment-reset-schedule
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### environment-reset-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::environment-reset-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## passkeycomplete-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::passkeycomplete-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeycomplete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::passkeycomplete
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-passkeycomplete

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-passkeycomplete
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeycomplete-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::passkeycomplete-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## passkeyoptions-role

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/role:Role::passkeyoptions-role
- Type: aws:iam/role:Role
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeyoptions

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/function:Function::passkeyoptions
- Type: aws:lambda/function:Function
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

#### allow-passkeyoptions

- Logical identity: urn:pulumi:preview::offline-preview::aws:lambda/permission:Permission::allow-passkeyoptions
- Type: aws:lambda/permission:Permission
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeyoptions-logs

- Logical identity: urn:pulumi:preview::offline-preview::aws:iam/rolePolicyAttachment:RolePolicyAttachment::passkeyoptions-logs
- Type: aws:iam/rolePolicyAttachment:RolePolicyAttachment
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## email-http-api

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/api:Api::email-http-api
- Type: aws:apigatewayv2/api:Api
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### email-stage

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/stage:Stage::email-stage
- Type: aws:apigatewayv2/stage:Stage
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### app-config-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::app-config-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-complete-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-email-complete-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-start-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-email-start-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-complete-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-passkey-complete-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-start-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-passkey-start-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-complete-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-password-complete-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-set-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-password-set-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-start-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-password-start-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-refresh-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-refresh-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-status-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::auth-status-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeycomplete-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::passkeycomplete-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeyoptions-integration

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/integration:Integration::passkeyoptions-integration
- Type: aws:apigatewayv2/integration:Integration
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### app-config-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::app-config-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-complete-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-email-complete-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-email-start-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-email-start-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-complete-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-passkey-complete-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-passkey-start-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-passkey-start-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-complete-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-password-complete-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-set-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-password-set-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-password-start-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-password-start-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-refresh-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-refresh-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### auth-status-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::auth-status-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeycomplete-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::passkeycomplete-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### passkeyoptions-route

- Logical identity: urn:pulumi:preview::offline-preview::aws:apigatewayv2/route:Route::passkeyoptions-route
- Type: aws:apigatewayv2/route:Route
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

## environment-reset-sweep

- Logical identity: urn:pulumi:preview::offline-preview::aws:cloudwatch/eventRule:EventRule::environment-reset-sweep
- Type: aws:cloudwatch/eventRule:EventRule
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked

### environment-reset-target

- Logical identity: urn:pulumi:preview::offline-preview::aws:cloudwatch/eventTarget:EventTarget::environment-reset-target
- Type: aws:cloudwatch/eventTarget:EventTarget
- Provider ID: 
- Owner: go.apexfission.aws.auth
- Feature: authentication
- Planned action: declared
- Presence: unknown
- Source: 
- Observed: not checked
- Reason: unchecked
