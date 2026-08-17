# Asset


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**created_by** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**environments** | [**Dict[str, Environment]**](Environment.md) |  | [optional] 
**external_links** | [**List[AssetExternalLink]**](AssetExternalLink.md) |  | [optional] 
**has_run_history** | **bool** |  | [optional] 
**id** | **str** |  | [optional] 
**is_stub** | **bool** |  | [optional] 
**last_sync_at** | **str** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**mrn** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**parent_mrn** | **str** |  | [optional] 
**providers** | **List[str]** |  | [optional] 
**query** | **str** |  | [optional] 
**query_language** | **str** |  | [optional] 
**var_schema** | **Dict[str, str]** |  | [optional] 
**sources** | [**List[AssetSource]**](AssetSource.md) |  | [optional] 
**tags** | **List[str]** |  | [optional] 
**terms** | **List[str]** | Terms carries glossary term names a discovery assigned to this asset. It is input only: the links live in asset_terms, so reading an asset back from the database never fills this in. | [optional] 
**type** | **str** |  | [optional] 
**updated_at** | **str** |  | [optional] 
**user_description** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.asset import Asset

# TODO update the JSON string below
json = "{}"
# create an instance of Asset from a JSON string
asset_instance = Asset.from_json(json)
# print the JSON string representation of the object
print(Asset.to_json())

# convert the object into a dict
asset_dict = asset_instance.to_dict()
# create an instance of Asset from a dict
asset_from_dict = Asset.from_dict(asset_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


