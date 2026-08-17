# RunCreateAssetRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**external_links** | **List[Dict[str, str]]** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**providers** | **List[str]** |  | [optional] 
**var_schema** | **Dict[str, object]** |  | [optional] 
**sources** | [**List[AssetSource]**](AssetSource.md) |  | [optional] 
**tags** | **List[str]** |  | [optional] 
**terms** | **List[str]** | Terms are the names of glossary terms assigned to this asset. | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.run_create_asset_request import RunCreateAssetRequest

# TODO update the JSON string below
json = "{}"
# create an instance of RunCreateAssetRequest from a JSON string
run_create_asset_request_instance = RunCreateAssetRequest.from_json(json)
# print the JSON string representation of the object
print(RunCreateAssetRequest.to_json())

# convert the object into a dict
run_create_asset_request_dict = run_create_asset_request_instance.to_dict()
# create an instance of RunCreateAssetRequest from a dict
run_create_asset_request_from_dict = RunCreateAssetRequest.from_dict(run_create_asset_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


