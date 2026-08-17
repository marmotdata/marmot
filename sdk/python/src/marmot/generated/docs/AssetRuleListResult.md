# AssetRuleListResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_rules** | [**List[AssetRule]**](AssetRule.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_rule_list_result import AssetRuleListResult

# TODO update the JSON string below
json = "{}"
# create an instance of AssetRuleListResult from a JSON string
asset_rule_list_result_instance = AssetRuleListResult.from_json(json)
# print the JSON string representation of the object
print(AssetRuleListResult.to_json())

# convert the object into a dict
asset_rule_list_result_dict = asset_rule_list_result_instance.to_dict()
# create an instance of AssetRuleListResult from a dict
asset_rule_list_result_from_dict = AssetRuleListResult.from_dict(asset_rule_list_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


