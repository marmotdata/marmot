# UIConfigResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**allow_unencrypted** | **bool** |  | [optional] 
**banner** | [**BannerResponse**](BannerResponse.md) |  | [optional] 
**encryption_configured** | **bool** |  | [optional] 
**table_preview_enabled** | **bool** |  | [optional] 

## Example

```python
from marmot.generated.models.ui_config_response import UIConfigResponse

# TODO update the JSON string below
json = "{}"
# create an instance of UIConfigResponse from a JSON string
ui_config_response_instance = UIConfigResponse.from_json(json)
# print the JSON string representation of the object
print(UIConfigResponse.to_json())

# convert the object into a dict
ui_config_response_dict = ui_config_response_instance.to_dict()
# create an instance of UIConfigResponse from a dict
ui_config_response_from_dict = UIConfigResponse.from_dict(ui_config_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


