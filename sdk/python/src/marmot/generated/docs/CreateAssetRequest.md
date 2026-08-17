# CreateAssetRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**environments** | [**Dict[str, Environment]**](Environment.md) |  | [optional] 
**external_links** | [**List[AssetExternalLink]**](AssetExternalLink.md) |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | 
**providers** | **List[str]** |  | 
**var_schema** | **Dict[str, str]** |  | [optional] 
**sources** | [**List[AssetSource]**](AssetSource.md) |  | [optional] 
**tags** | **List[str]** |  | [optional] 
**type** | **str** |  | 

## Example

```python
from marmot.generated.models.create_asset_request import CreateAssetRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateAssetRequest from a JSON string
create_asset_request_instance = CreateAssetRequest.from_json(json)
# print the JSON string representation of the object
print(CreateAssetRequest.to_json())

# convert the object into a dict
create_asset_request_dict = create_asset_request_instance.to_dict()
# create an instance of CreateAssetRequest from a dict
create_asset_request_from_dict = CreateAssetRequest.from_dict(create_asset_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


