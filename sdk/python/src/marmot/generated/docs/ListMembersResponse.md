# ListMembersResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**members** | [**List[TeamMemberWithUser]**](TeamMemberWithUser.md) |  | [optional] 

## Example

```python
from marmot.generated.models.list_members_response import ListMembersResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ListMembersResponse from a JSON string
list_members_response_instance = ListMembersResponse.from_json(json)
# print the JSON string representation of the object
print(ListMembersResponse.to_json())

# convert the object into a dict
list_members_response_dict = list_members_response_instance.to_dict()
# create an instance of ListMembersResponse from a dict
list_members_response_from_dict = ListMembersResponse.from_dict(list_members_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


