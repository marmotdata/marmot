# ValidateConfigResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**errors** | [**List[ValidationErrorDetail]**](ValidationErrorDetail.md) |  | [optional] 
**valid** | **bool** |  | [optional] 

## Example

```python
from marmot.generated.models.validate_config_response import ValidateConfigResponse

# TODO update the JSON string below
json = "{}"
# create an instance of ValidateConfigResponse from a JSON string
validate_config_response_instance = ValidateConfigResponse.from_json(json)
# print the JSON string representation of the object
print(ValidateConfigResponse.to_json())

# convert the object into a dict
validate_config_response_dict = validate_config_response_instance.to_dict()
# create an instance of ValidateConfigResponse from a dict
validate_config_response_from_dict = ValidateConfigResponse.from_dict(validate_config_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


