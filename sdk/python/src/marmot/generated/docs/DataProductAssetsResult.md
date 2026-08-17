# DataProductAssetsResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_ids** | **List[str]** |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product_assets_result import DataProductAssetsResult

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductAssetsResult from a JSON string
data_product_assets_result_instance = DataProductAssetsResult.from_json(json)
# print the JSON string representation of the object
print(DataProductAssetsResult.to_json())

# convert the object into a dict
data_product_assets_result_dict = data_product_assets_result_instance.to_dict()
# create an instance of DataProductAssetsResult from a dict
data_product_assets_result_from_dict = DataProductAssetsResult.from_dict(data_product_assets_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


