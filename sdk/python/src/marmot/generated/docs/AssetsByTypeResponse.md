# AssetsByTypeResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets** | **Dict[str, int]** |  | [optional] 

## Example

```python
from marmot.generated.models.assets_by_type_response import AssetsByTypeResponse

# TODO update the JSON string below
json = "{}"
# create an instance of AssetsByTypeResponse from a JSON string
assets_by_type_response_instance = AssetsByTypeResponse.from_json(json)
# print the JSON string representation of the object
print(AssetsByTypeResponse.to_json())

# convert the object into a dict
assets_by_type_response_dict = assets_by_type_response_instance.to_dict()
# create an instance of AssetsByTypeResponse from a dict
assets_by_type_response_from_dict = AssetsByTypeResponse.from_dict(assets_by_type_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


