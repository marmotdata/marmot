# AssetSource


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**last_sync_at** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**priority** | **int** |  | [optional] 
**properties** | **Dict[str, object]** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_source import AssetSource

# TODO update the JSON string below
json = "{}"
# create an instance of AssetSource from a JSON string
asset_source_instance = AssetSource.from_json(json)
# print the JSON string representation of the object
print(AssetSource.to_json())

# convert the object into a dict
asset_source_dict = asset_source_instance.to_dict()
# create an instance of AssetSource from a dict
asset_source_from_dict = AssetSource.from_dict(asset_source_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


