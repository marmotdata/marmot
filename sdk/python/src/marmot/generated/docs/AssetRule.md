# AssetRule


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**created_by** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**is_enabled** | **bool** |  | [optional] 
**last_reconciled_at** | **str** |  | [optional] 
**links** | [**List[AssetRuleExternalLink]**](AssetRuleExternalLink.md) |  | [optional] 
**membership_count** | **int** |  | [optional] 
**metadata_field** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**pattern_type** | **str** |  | [optional] 
**pattern_value** | **str** |  | [optional] 
**priority** | **int** |  | [optional] 
**query_expression** | **str** |  | [optional] 
**reconciliation_hash** | **str** |  | [optional] 
**rule_type** | [**RuleType**](RuleType.md) |  | [optional] 
**term_ids** | **List[str]** |  | [optional] 
**updated_at** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_rule import AssetRule

# TODO update the JSON string below
json = "{}"
# create an instance of AssetRule from a JSON string
asset_rule_instance = AssetRule.from_json(json)
# print the JSON string representation of the object
print(AssetRule.to_json())

# convert the object into a dict
asset_rule_dict = asset_rule_instance.to_dict()
# create an instance of AssetRule from a dict
asset_rule_from_dict = AssetRule.from_dict(asset_rule_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


