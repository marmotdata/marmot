# UpdateAssetRuleRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**description** | **str** |  | [optional] 
**is_enabled** | **bool** |  | [optional] 
**links** | [**List[AssetRuleExternalLink]**](AssetRuleExternalLink.md) |  | [optional] 
**metadata_field** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**pattern_type** | **str** |  | [optional] 
**pattern_value** | **str** |  | [optional] 
**priority** | **int** |  | [optional] 
**query_expression** | **str** |  | [optional] 
**rule_type** | **str** |  | [optional] 
**term_ids** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.update_asset_rule_request import UpdateAssetRuleRequest

# TODO update the JSON string below
json = "{}"
# create an instance of UpdateAssetRuleRequest from a JSON string
update_asset_rule_request_instance = UpdateAssetRuleRequest.from_json(json)
# print the JSON string representation of the object
print(UpdateAssetRuleRequest.to_json())

# convert the object into a dict
update_asset_rule_request_dict = update_asset_rule_request_instance.to_dict()
# create an instance of UpdateAssetRuleRequest from a dict
update_asset_rule_request_from_dict = UpdateAssetRuleRequest.from_dict(update_asset_rule_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


