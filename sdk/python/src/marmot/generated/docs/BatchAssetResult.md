# BatchAssetResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset** | **object** |  | [optional] 
**error** | **str** |  | [optional] 
**mrn** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**provider** | **str** |  | [optional] 
**status** | **str** |  | [optional] 
**type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.batch_asset_result import BatchAssetResult

# TODO update the JSON string below
json = "{}"
# create an instance of BatchAssetResult from a JSON string
batch_asset_result_instance = BatchAssetResult.from_json(json)
# print the JSON string representation of the object
print(BatchAssetResult.to_json())

# convert the object into a dict
batch_asset_result_dict = batch_asset_result_instance.to_dict()
# create an instance of BatchAssetResult from a dict
batch_asset_result_from_dict = BatchAssetResult.from_dict(batch_asset_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


