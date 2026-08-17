# V1RolesCreateRoleRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**permission_ids** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.v1_roles_create_role_request import V1RolesCreateRoleRequest

# TODO update the JSON string below
json = "{}"
# create an instance of V1RolesCreateRoleRequest from a JSON string
v1_roles_create_role_request_instance = V1RolesCreateRoleRequest.from_json(json)
# print the JSON string representation of the object
print(V1RolesCreateRoleRequest.to_json())

# convert the object into a dict
v1_roles_create_role_request_dict = v1_roles_create_role_request_instance.to_dict()
# create an instance of V1RolesCreateRoleRequest from a dict
v1_roles_create_role_request_from_dict = V1RolesCreateRoleRequest.from_dict(v1_roles_create_role_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


