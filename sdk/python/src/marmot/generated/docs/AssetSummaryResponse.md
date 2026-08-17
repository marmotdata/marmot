# AssetSummaryResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**providers** | **Dict[str, int]** |  | [optional] 
**tags** | **Dict[str, int]** |  | [optional] 
**types** | [**Dict[str, AssetTypeSummary]**](AssetTypeSummary.md) |  | [optional] 

## Example

```python
from marmot.generated.models.asset_summary_response import AssetSummaryResponse

# TODO update the JSON string below
json = "{}"
# create an instance of AssetSummaryResponse from a JSON string
asset_summary_response_instance = AssetSummaryResponse.from_json(json)
# print the JSON string representation of the object
print(AssetSummaryResponse.to_json())

# convert the object into a dict
asset_summary_response_dict = asset_summary_response_instance.to_dict()
# create an instance of AssetSummaryResponse from a dict
asset_summary_response_from_dict = AssetSummaryResponse.from_dict(asset_summary_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


