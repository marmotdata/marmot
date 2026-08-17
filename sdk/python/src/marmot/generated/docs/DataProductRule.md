# DataProductRule


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**data_product_id** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**is_enabled** | **bool** |  | [optional] 
**matched_asset_count** | **int** |  | [optional] 
**metadata_field** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**pattern_type** | **str** |  | [optional] 
**pattern_value** | **str** |  | [optional] 
**priority** | **int** |  | [optional] 
**query_expression** | **str** |  | [optional] 
**rule_type** | [**DataProductRuleType**](DataProductRuleType.md) |  | [optional] 
**updated_at** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product_rule import DataProductRule

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductRule from a JSON string
data_product_rule_instance = DataProductRule.from_json(json)
# print the JSON string representation of the object
print(DataProductRule.to_json())

# convert the object into a dict
data_product_rule_dict = data_product_rule_instance.to_dict()
# create an instance of DataProductRule from a dict
data_product_rule_from_dict = DataProductRule.from_dict(data_product_rule_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


