# AddMemberRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**role** | **str** |  | [optional] 
**user_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.add_member_request import AddMemberRequest

# TODO update the JSON string below
json = "{}"
# create an instance of AddMemberRequest from a JSON string
add_member_request_instance = AddMemberRequest.from_json(json)
# print the JSON string representation of the object
print(AddMemberRequest.to_json())

# convert the object into a dict
add_member_request_dict = add_member_request_instance.to_dict()
# create an instance of AddMemberRequest from a dict
add_member_request_from_dict = AddMemberRequest.from_dict(add_member_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


