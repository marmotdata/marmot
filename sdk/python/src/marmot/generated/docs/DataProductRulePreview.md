# DataProductRulePreview


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_count** | **int** |  | [optional] 
**asset_ids** | **List[str]** |  | [optional] 
**errors** | **List[str]** |  | [optional] 

## Example

```python
from marmot.generated.models.data_product_rule_preview import DataProductRulePreview

# TODO update the JSON string below
json = "{}"
# create an instance of DataProductRulePreview from a JSON string
data_product_rule_preview_instance = DataProductRulePreview.from_json(json)
# print the JSON string representation of the object
print(DataProductRulePreview.to_json())

# convert the object into a dict
data_product_rule_preview_dict = data_product_rule_preview_instance.to_dict()
# create an instance of DataProductRulePreview from a dict
data_product_rule_preview_from_dict = DataProductRulePreview.from_dict(data_product_rule_preview_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


