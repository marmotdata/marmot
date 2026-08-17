# CreateUserInput


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**name** | **str** |  | 
**oauth_provider** | **str** |  | [optional] 
**oauth_provider_data** | **Dict[str, object]** |  | [optional] 
**oauth_provider_id** | **str** |  | [optional] 
**password** | **str** |  | [optional] 
**profile_picture** | **str** |  | [optional] 
**role_names** | **List[str]** |  | 
**username** | **str** |  | 

## Example

```python
from marmot.generated.models.create_user_input import CreateUserInput

# TODO update the JSON string below
json = "{}"
# create an instance of CreateUserInput from a JSON string
create_user_input_instance = CreateUserInput.from_json(json)
# print the JSON string representation of the object
print(CreateUserInput.to_json())

# convert the object into a dict
create_user_input_dict = create_user_input_instance.to_dict()
# create an instance of CreateUserInput from a dict
create_user_input_from_dict = CreateUserInput.from_dict(create_user_input_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


