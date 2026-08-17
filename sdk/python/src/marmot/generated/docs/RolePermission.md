# RolePermission


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**action** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**resource_type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.role_permission import RolePermission

# TODO update the JSON string below
json = "{}"
# create an instance of RolePermission from a JSON string
role_permission_instance = RolePermission.from_json(json)
# print the JSON string representation of the object
print(RolePermission.to_json())

# convert the object into a dict
role_permission_dict = role_permission_instance.to_dict()
# create an instance of RolePermission from a dict
role_permission_from_dict = RolePermission.from_dict(role_permission_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


