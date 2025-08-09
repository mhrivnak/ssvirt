We need to change the API schemas to adhere to the vmware cloud director APIs. It looks like you didn't follow its documentation very closely.

Don't worry about migrating existing databases; we can just re-install.

Every record of any type must have an "id" field which is a string. It is a URN formatted like this: "urn:vcloud:$TYPE:$UUID".

Types include:
- user
- org
- role

## Users

Using this documentation for Users: https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/user/
User API must support these fields:

`
{
    "username": "string",
    "fullName": "string",
    "description": "string",
    "id": "string",
    "roleEntityRefs": [
        {
            "name": "string",
            "id": "string"
        }
    ],
    "orgEntityRef": {
        "name": "string",
        "id": "string"
    },
    "password": "string",
    "deployedVmQuota": 0,
    "storedVmQuota": 0,
    "email": "string",
    "nameInSource": "string",
    "enabled": false,
    "isGroupRole": false,
    "providerType": "string",
    "locked": false,
    "stranded": false
}
`
each item in "roleEntityRefs" has the ID and name of an Organization.

each item in "orgEntityRefs" has the ID and name of a Role.

User bulk queries should be served at /cloudapi/1.0.0/users
Single user queries should be served at /cloudapi/1.0.0/users/$ID

## Roles

Using this documentation for Roles: https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/roles/
Role API must support these fields:

`
{
    "name": "string",
    "id": "string",
    "description": "string",
    "bundleKey": "string",
    "readOnly": false
}
`

Role bulk queries should be served at /cloudapi/1.0.0/roles
Single role queries should be served at /cloudapi/1.0.0/roles/$ID

Roles should all be read-only and can have a blank description and bundleKey.

## Orgs

Using this documentation for Orgs: https://developer.broadcom.com/xapis/vmware-cloud-director-openapi/latest/org/
Role API must support these fields:

`
{
    "id": "string",
    "name": "string",
    "displayName": "string",
    "description": "string",
    "isEnabled": false,
    "orgVdcCount": 0,
    "catalogCount": 0,
    "vappCount": 0,
    "runningVMCount": 0,
    "userCount": 0,
    "diskCount": 0,
    "managedBy": {
        "name": "string",
        "id": "string"
    },
    "canManageOrgs": false,
    "canPublish": false,
    "maskedEventTaskUsername": "string",
    "directlyManagedOrgCount": 0
}
`

Org bulk queries should be served at /cloudapi/1.0.0/orgs
Single org queries should be served at /cloudapi/1.0.0/orgs/$ID

We need to populate the Roles with exactly these items, and have constant values in code that we can use later to determine if a user has permissions to take an action or view data. We'll add that logic later.
- System Administrator
- Organization Administrator
- vApp User

There should be a default organization called "Provider".

The initial admin user should have the System Administrator role and be a member of the Provider organization.