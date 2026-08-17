# ValidateConfigRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**config** | **Dict[str, object]** |  | [optional] 
**plugin_id** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.validate_config_request import ValidateConfigRequest

# TODO update the JSON string below
json = "{}"
# create an instance of ValidateConfigRequest from a JSON string
validate_config_request_instance = ValidateConfigRequest.from_json(json)
# print the JSON string representation of the object
print(ValidateConfigRequest.to_json())

# convert the object into a dict
validate_config_request_dict = validate_config_request_instance.to_dict()
# create an instance of ValidateConfigRequest from a dict
validate_config_request_from_dict = ValidateConfigRequest.from_dict(validate_config_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


