# AssetSearchResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets** | [**List[Asset]**](Asset.md) |  | [optional] 
**filters** | [**AvailableFilters**](AvailableFilters.md) |  | [optional] 
**limit** | **int** |  | [optional] 
**offset** | **int** |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_search_response import AssetSearchResponse

# TODO update the JSON string below
json = "{}"
# create an instance of AssetSearchResponse from a JSON string
asset_search_response_instance = AssetSearchResponse.from_json(json)
# print the JSON string representation of the object
print(AssetSearchResponse.to_json())

# convert the object into a dict
asset_search_response_dict = asset_search_response_instance.to_dict()
# create an instance of AssetSearchResponse from a dict
asset_search_response_from_dict = AssetSearchResponse.from_dict(asset_search_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


