# UpdateUserInput


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**active** | **bool** |  | [optional] 
**email** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**password** | **str** |  | [optional] 
**preferences** | **Dict[str, object]** |  | [optional] 
**profile_picture** | **str** |  | [optional] 
**role_names** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.update_user_input import UpdateUserInput

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateUserInput from a JSON string
update_user_input_instance = UpdateUserInput.from_json(json)
# print the JSON string representation of the object
print(UpdateUserInput.to_json())

# convert the object into a dict
update_user_input_dict = update_user_input_instance.to_dict()
# create an instance of UpdateUserInput from a dict
update_user_input_from_dict = UpdateUserInput.from_dict(update_user_input_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


