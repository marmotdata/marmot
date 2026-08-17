# UpdateMemberRoleRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**role** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.update_member_role_request import UpdateMemberRoleRequest

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateMemberRoleRequest from a JSON string
update_member_role_request_instance = UpdateMemberRoleRequest.from_json(json)
# print the JSON string representation of the object
print(UpdateMemberRoleRequest.to_json())

# convert the object into a dict
update_member_role_request_dict = update_member_role_request_instance.to_dict()
# create an instance of UpdateMemberRoleRequest from a dict
update_member_role_request_from_dict = UpdateMemberRoleRequest.from_dict(update_member_role_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


