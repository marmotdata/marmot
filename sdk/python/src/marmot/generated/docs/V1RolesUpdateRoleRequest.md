# V1RolesUpdateRoleRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**name** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.v1_roles_update_role_request import V1RolesUpdateRoleRequest

# TODO update the JSON string below
json = "{}"
# create an instance of V1RolesUpdateRoleRequest from a JSON string
v1_roles_update_role_request_instance = V1RolesUpdateRoleRequest.from_json(json)
# print the JSON string representation of the object
print(V1RolesUpdateRoleRequest.to_json())

# convert the object into a dict
v1_roles_update_role_request_dict = v1_roles_update_role_request_instance.to_dict()
# create an instance of V1RolesUpdateRoleRequest from a dict
v1_roles_update_role_request_from_dict = V1RolesUpdateRoleRequest.from_dict(v1_roles_update_role_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


