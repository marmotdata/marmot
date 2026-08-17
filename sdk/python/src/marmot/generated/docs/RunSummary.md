# RunSummary


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets_created** | **int** |  | [optional] 
**assets_deleted** | **int** |  | [optional] 
**assets_terms_linked** | **int** |  | [optional] 
**assets_updated** | **int** |  | [optional] 
**documentation_added** | **int** |  | [optional] 
**duration_seconds** | **int** |  | [optional] 
**errors_count** | **int** |  | [optional] 
**glossary_terms_created** | **int** | Glossary counts are absent from runs that predate them, and from sources that curate no business terms. | [optional] 
**glossary_terms_updated** | **int** |  | [optional] 
**lineage_created** | **int** |  | [optional] 
**lineage_updated** | **int** |  | [optional] 
**total_entities** | **int** |  | [optional] 

## Example

```python
from marmot.generated.models.run_summary import RunSummary

# TODO update the JSON string below
json = "{}"
# create an instance of RunSummary from a JSON string
run_summary_instance = RunSummary.from_json(json)
# print the JSON string representation of the object
print(RunSummary.to_json())

# convert the object into a dict
run_summary_dict = run_summary_instance.to_dict()
# create an instance of RunSummary from a dict
run_summary_from_dict = RunSummary.from_dict(run_summary_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


