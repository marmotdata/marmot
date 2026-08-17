# V1RolesReplacePermissionsRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**permission_ids** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.v1_roles_replace_permissions_request import V1RolesReplacePermissionsRequest

# TODO update the JSON string below
json = "{}"
# create an instance of V1RolesReplacePermissionsRequest from a JSON string
v1_roles_replace_permissions_request_instance = V1RolesReplacePermissionsRequest.from_json(json)
# print the JSON string representation of the object
print(V1RolesReplacePermissionsRequest.to_json())

# convert the object into a dict
v1_roles_replace_permissions_request_dict = v1_roles_replace_permissions_request_instance.to_dict()
# create an instance of V1RolesReplacePermissionsRequest from a dict
v1_roles_replace_permissions_request_from_dict = V1RolesReplacePermissionsRequest.from_dict(v1_roles_replace_permissions_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


