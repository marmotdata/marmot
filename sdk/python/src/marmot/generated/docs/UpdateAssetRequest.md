# UpdateAssetRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**environments** | [**Dict[str, Environment]**](Environment.md) |  | [optional] 
**external_links** | [**List[AssetExternalLink]**](AssetExternalLink.md) |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**providers** | **List[str]** |  | [optional] 
**var_schema** | **Dict[str, str]** |  | [optional] 
**sources** | [**List[AssetSource]**](AssetSource.md) |  | [optional] 
**tags** | **List[str]** |  | [optional] 
**type** | **str** |  | [optional] 
**user_description** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.update_asset_request import UpdateAssetRequest

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateAssetRequest from a JSON string
update_asset_request_instance = UpdateAssetRequest.from_json(json)
# print the JSON string representation of the object
print(UpdateAssetRequest.to_json())

# convert the object into a dict
update_asset_request_dict = update_asset_request_instance.to_dict()
# create an instance of UpdateAssetRequest from a dict
update_asset_request_from_dict = UpdateAssetRequest.from_dict(update_asset_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


