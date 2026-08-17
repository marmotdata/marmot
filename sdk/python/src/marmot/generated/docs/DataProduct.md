# DataProduct


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_count** | **int** |  | [optional] 
**created_at** | **str** |  | [optional] 
**created_by** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**icon_url** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**manual_asset_count** | **int** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**owners** | [**List[DataProductOwner]**](DataProductOwner.md) |  | [optional] 
**rule_asset_count** | **int** |  | [optional] 
**rules** | [**List[DataProductRule]**](DataProductRule.md) |  | [optional] 
**tags** | **List[str]** |  | [optional] 
**updated_at** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product import DataProduct

# TODO update the JSON string below
json = "{}"
# create an instance of DataProduct from a JSON string
data_product_instance = DataProduct.from_json(json)
# print the JSON string representation of the object
print(DataProduct.to_json())

# convert the object into a dict
data_product_dict = data_product_instance.to_dict()
# create an instance of DataProduct from a dict
data_product_from_dict = DataProduct.from_dict(data_product_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


