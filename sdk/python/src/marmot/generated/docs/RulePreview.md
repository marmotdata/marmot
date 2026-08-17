# RulePreview


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_count** | **int** |  | [optional] 
**asset_ids** | **List[str]** |  | [optional] 
**errors** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.rule_preview import RulePreview

# TODO update the JSON string below
json = "{}"
# create an instance of RulePreview from a JSON string
rule_preview_instance = RulePreview.from_json(json)
# print the JSON string representation of the object
print(RulePreview.to_json())

# convert the object into a dict
rule_preview_dict = rule_preview_instance.to_dict()
# create an instance of RulePreview from a dict
rule_preview_from_dict = RulePreview.from_dict(rule_preview_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


