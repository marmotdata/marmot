# ValidationErrorDetail


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**var_field** | **str** |  | [optional] 
**message** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.validation_error_detail import ValidationErrorDetail

# TODO update the JSON string below
json = "{}"
# create an instance of ValidationErrorDetail from a JSON string
validation_error_detail_instance = ValidationErrorDetail.from_json(json)
# print the JSON string representation of the object
print(ValidationErrorDetail.to_json())

# convert the object into a dict
validation_error_detail_dict = validation_error_detail_instance.to_dict()
# create an instance of ValidationErrorDetail from a dict
validation_error_detail_from_dict = ValidationErrorDetail.from_dict(validation_error_detail_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


