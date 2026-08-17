# DataProductRulesResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**rules** | [**List[DataProductRule]**](DataProductRule.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product_rules_response import DataProductRulesResponse

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductRulesResponse from a JSON string
data_product_rules_response_instance = DataProductRulesResponse.from_json(json)
# print the JSON string representation of the object
print(DataProductRulesResponse.to_json())

# convert the object into a dict
data_product_rules_response_dict = data_product_rules_response_instance.to_dict()
# create an instance of DataProductRulesResponse from a dict
data_product_rules_response_from_dict = DataProductRulesResponse.from_dict(data_product_rules_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


