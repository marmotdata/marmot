# DataProductResolvedAssets


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**all_assets** | **List[str]** |  | [optional] 
**dynamic_assets** | **List[str]** |  | [optional] 
**manual_assets** | **List[str]** |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product_resolved_assets import DataProductResolvedAssets

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductResolvedAssets from a JSON string
data_product_resolved_assets_instance = DataProductResolvedAssets.from_json(json)
# print the JSON string representation of the object
print(DataProductResolvedAssets.to_json())

# convert the object into a dict
data_product_resolved_assets_dict = data_product_resolved_assets_instance.to_dict()
# create an instance of DataProductResolvedAssets from a dict
data_product_resolved_assets_from_dict = DataProductResolvedAssets.from_dict(data_product_resolved_assets_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


