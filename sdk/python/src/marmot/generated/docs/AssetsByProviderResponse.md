# AssetsByProviderResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets** | **Dict[str, int]** |  | [optional] 

## Example

```python
from marmot.generated.models.assets_by_provider_response import AssetsByProviderResponse

# TODO update the JSON string below
json = "{}"
# create an instance of AssetsByProviderResponse from a JSON string
assets_by_provider_response_instance = AssetsByProviderResponse.from_json(json)
# print the JSON string representation of the object
print(AssetsByProviderResponse.to_json())

# convert the object into a dict
assets_by_provider_response_dict = assets_by_provider_response_instance.to_dict()
# create an instance of AssetsByProviderResponse from a dict
assets_by_provider_response_from_dict = AssetsByProviderResponse.from_dict(assets_by_provider_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


