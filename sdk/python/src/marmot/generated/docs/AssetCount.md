# AssetCount


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_id** | **str** |  | [optional] 
**asset_name** | **str** |  | [optional] 
**asset_provider** | **str** |  | [optional] 
**asset_type** | **str** |  | [optional] 
**count** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_count import AssetCount

# TODO update the JSON string below
json = "{}"
# create an instance of AssetCount from a JSON string
asset_count_instance = AssetCount.from_json(json)
# print the JSON string representation of the object
print(AssetCount.to_json())

# convert the object into a dict
asset_count_dict = asset_count_instance.to_dict()
# create an instance of AssetCount from a dict
asset_count_from_dict = AssetCount.from_dict(asset_count_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


