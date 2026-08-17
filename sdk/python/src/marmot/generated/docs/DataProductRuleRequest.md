# DataProductRuleRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**is_enabled** | **bool** |  | [optional] 
**metadata_field** | **str** |  | [optional] 
**name** | **str** |  | 
**pattern_type** | **str** |  | [optional] 
**pattern_value** | **str** |  | [optional] 
**priority** | **int** |  | [optional] 
**query_expression** | **str** |  | [optional] 
**rule_type** | **str** |  | 

## Example

```python
from marmot.generated.models.data_product_rule_request import DataProductRuleRequest

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductRuleRequest from a JSON string
data_product_rule_request_instance = DataProductRuleRequest.from_json(json)
# print the JSON string representation of the object
print(DataProductRuleRequest.to_json())

# convert the object into a dict
data_product_rule_request_dict = data_product_rule_request_instance.to_dict()
# create an instance of DataProductRuleRequest from a dict
data_product_rule_request_from_dict = DataProductRuleRequest.from_dict(data_product_rule_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


