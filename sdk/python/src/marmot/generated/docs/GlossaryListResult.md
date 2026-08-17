# GlossaryListResult


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**terms** | [**List[GlossaryTerm]**](GlossaryTerm.md) |  | [optional] 
**total** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.glossary_list_result import GlossaryListResult

# TODO update the JSON string below
json = "{}"
# create an instance of GlossaryListResult from a JSON string
glossary_list_result_instance = GlossaryListResult.from_json(json)
# print the JSON string representation of the object
print(GlossaryListResult.to_json())

# convert the object into a dict
glossary_list_result_dict = glossary_list_result_instance.to_dict()
# create an instance of GlossaryListResult from a dict
glossary_list_result_from_dict = GlossaryListResult.from_dict(glossary_list_result_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


