# GlossaryTerm


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**definition** | **str** | Definition is what the last run wrote. On a term a person has worded themselves the read path serves UserDefinition here instead, so a caller always gets the wording the catalog stands behind. | [optional] 
**deleted_at** | **str** |  | [optional] 
**description** | **str** |  | [optional] 
**id** | **str** |  | [optional] 
**metadata** | **Dict[str, object]** |  | [optional] 
**name** | **str** |  | [optional] 
**owners** | [**List[GlossaryOwner]**](GlossaryOwner.md) |  | [optional] 
**parent_term_id** | **str** |  | [optional] 
**tags** | **List[str]** |  | [optional] 
**updated_at** | **str** |  | [optional] 
**user_definition** | **str** | UserDefinition is the wording a person gave the term. Ingestion reads it and never writes it, so it survives every run. | [optional] 

## Example

```python
from marmot.generated.models.glossary_term import GlossaryTerm

# TODO update the JSON string below
json = "{}"
# create an instance of GlossaryTerm from a JSON string
glossary_term_instance = GlossaryTerm.from_json(json)
# print the JSON string representation of the object
print(GlossaryTerm.to_json())

# convert the object into a dict
glossary_term_dict = glossary_term_instance.to_dict()
# create an instance of GlossaryTerm from a dict
glossary_term_from_dict = GlossaryTerm.from_dict(glossary_term_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


