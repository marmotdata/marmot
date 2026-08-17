# DataProductListResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**data_products** | [**List[DataProduct]**](DataProduct.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product_list_result import DataProductListResult

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductListResult from a JSON string
data_product_list_result_instance = DataProductListResult.from_json(json)
# print the JSON string representation of the object
print(DataProductListResult.to_json())

# convert the object into a dict
data_product_list_result_dict = data_product_list_result_instance.to_dict()
# create an instance of DataProductListResult from a dict
data_product_list_result_from_dict = DataProductListResult.from_dict(data_product_list_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


