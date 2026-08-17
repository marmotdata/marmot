# AssetTypeSummary


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**count** | **int** |  | [optional] 
**service** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_type_summary import AssetTypeSummary

# TODO update the JSON string below
json = "{}"
# create an instance of AssetTypeSummary from a JSON string
asset_type_summary_instance = AssetTypeSummary.from_json(json)
# print the JSON string representation of the object
print(AssetTypeSummary.to_json())

# convert the object into a dict
asset_type_summary_dict = asset_type_summary_instance.to_dict()
# create an instance of AssetTypeSummary from a dict
asset_type_summary_from_dict = AssetTypeSummary.from_dict(asset_type_summary_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


