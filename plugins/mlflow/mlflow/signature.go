package mlflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// logModelHistoryTag is the run tag MLflow appends to every time a model is
// logged to the run. Each entry carries the model's signature, so no
// artifact read is needed when the tag is present.
const logModelHistoryTag = "mlflow.log-model.history"

// signatureInput is one entry of a model signature's inputs. Required is
// a pointer because MLflow only started writing it in 2.x, and an absent
// value means required.
type signatureInput struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required *bool  `json:"required"`
}

func (i signatureInput) required() bool {
	return i.Required == nil || *i.Required
}

// loggedModelEntry is one entry of the log-model history tag.
type loggedModelEntry struct {
	RunID        string         `json:"run_id"`
	ArtifactPath string         `json:"artifact_path"`
	Signature    modelSignature `json:"signature"`
}

// modelSignature is the signature block as MLflow serialises it, both in
// the history tag and in the MLmodel file: inputs is a JSON document
// stored as a string.
type modelSignature struct {
	Inputs json.RawMessage `json:"inputs"`
}

// mlModelFile is the part of an MLmodel file this plugin reads.
type mlModelFile struct {
	Signature modelSignature `json:"signature"`
}

// signatureColumns returns a model version's input features as columns.
// The run's log-model history tag is the cheapest place to find the
// signature; failing that, the MLmodel file is read from the logged
// model (MLflow 3, models:/ sources) or from the run's artifacts.
func (d *discovery) signatureColumns(ctx context.Context, version *modelVersion, r *run) []pluginsdk.Column {
	if version == nil {
		return nil
	}

	if r != nil {
		if inputs, ok := inputsFromHistory(r.Data.tag(logModelHistoryTag), version); ok {
			return featureColumns(inputs)
		}
	}

	file, err := d.readMLModel(ctx, version, r)
	if err != nil {
		log.Debug().Err(err).Str("model", version.Name).Str("version", version.Version).
			Msg("No model signature found, emitting no features")
		return nil
	}
	if len(file.Signature.Inputs) == 0 {
		log.Debug().Str("model", version.Name).Str("version", version.Version).
			Msg("MLmodel file has no signature, emitting no features")
		return nil
	}

	inputs, err := parseSignatureInputs(file.Signature.Inputs)
	if err != nil {
		log.Debug().Err(err).Str("model", version.Name).Str("version", version.Version).
			Msg("Could not parse the MLmodel signature, emitting no features")
		return nil
	}
	return featureColumns(inputs)
}

// inputsFromHistory finds the version's model in the run's log-model
// history and returns its signature inputs. A run may log several
// models, so the entry whose artifact path matches the version source
// wins over any other entry for the run.
func inputsFromHistory(tag string, version *modelVersion) ([]signatureInput, bool) {
	if tag == "" {
		return nil, false
	}

	var entries []loggedModelEntry
	if err := json.Unmarshal([]byte(tag), &entries); err != nil {
		log.Debug().Err(err).Str("model", version.Name).Msg("Could not parse the log-model history tag")
		return nil, false
	}

	wantedPath := runArtifactPath(version.Source)
	var match *loggedModelEntry
	for i := range entries {
		e := &entries[i]
		if e.RunID != version.RunID {
			continue
		}
		if wantedPath != "" && e.ArtifactPath == wantedPath {
			match = e
			break
		}
		if match == nil {
			match = e
		}
	}
	if match == nil || len(match.Signature.Inputs) == 0 {
		return nil, false
	}

	inputs, err := parseSignatureInputs(match.Signature.Inputs)
	if err != nil {
		log.Debug().Err(err).Str("model", version.Name).Msg("Could not parse the signature in the log-model history tag")
		return nil, false
	}
	return inputs, true
}

// readMLModel fetches and parses the MLmodel file of a model version.
func (d *discovery) readMLModel(ctx context.Context, version *modelVersion, r *run) (*mlModelFile, error) {
	body, err := d.fetchMLModel(ctx, version, r)
	if err != nil {
		return nil, err
	}

	var file mlModelFile
	if err := yaml.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("parsing MLmodel: %w", err)
	}
	return &file, nil
}

// fetchMLModel locates the MLmodel file from the version's source URI.
// Three shapes reach a registry: models:/<id> (MLflow 3 logged models),
// runs:/<run>/<path>, and a resolved mlflow-artifacts:/ path.
func (d *discovery) fetchMLModel(ctx context.Context, version *modelVersion, r *run) ([]byte, error) {
	source := version.Source
	switch {
	case strings.HasPrefix(source, "models:/"):
		modelID := strings.TrimPrefix(source, "models:/")
		logged, err := d.client.getLoggedModel(ctx, modelID)
		if err != nil {
			return nil, fmt.Errorf("fetching logged model %s: %w", modelID, err)
		}
		path, ok := proxiedArtifactPath(logged.Info.ArtifactURI)
		if !ok {
			return nil, fmt.Errorf("logged model %s stores artifacts at %q, which the tracking server does not serve", modelID, logged.Info.ArtifactURI)
		}
		return d.client.proxiedArtifact(ctx, path+"/MLmodel")

	case strings.HasPrefix(source, "runs:/"):
		runID := version.RunID
		if r != nil {
			runID = r.Info.RunID
		}
		path := "MLmodel"
		if artifactPath := runArtifactPath(source); artifactPath != "" {
			path = artifactPath + "/MLmodel"
		}
		return d.client.runArtifact(ctx, runID, path)

	default:
		path, ok := proxiedArtifactPath(source)
		if !ok {
			return nil, fmt.Errorf("model source %q is not served by the tracking server", source)
		}
		return d.client.proxiedArtifact(ctx, path+"/MLmodel")
	}
}

// runArtifactPath returns the artifact path inside a runs:/<run_id>/<path>
// source, or "" for any other shape.
func runArtifactPath(source string) string {
	rest, ok := strings.CutPrefix(source, "runs:/")
	if !ok {
		return ""
	}
	_, path, _ := strings.Cut(rest, "/")
	return strings.Trim(path, "/")
}

// proxiedArtifactPath returns the path below the server's artifact root
// for an mlflow-artifacts:/ URI. Both the bare and the host-qualified
// forms (mlflow-artifacts://host:port/path) appear in practice.
func proxiedArtifactPath(uri string) (string, bool) {
	rest, ok := strings.CutPrefix(uri, "mlflow-artifacts:")
	if !ok {
		return "", false
	}
	if strings.HasPrefix(rest, "//") {
		_, rest, _ = strings.Cut(strings.TrimPrefix(rest, "//"), "/")
	}
	return strings.Trim(rest, "/"), true
}

// parseSignatureInputs decodes a signature's inputs. MLflow stores them
// as a JSON array serialised into a string, but a bare array and a
// Python repr (single quotes, True/False/None) have both been seen in
// the wild, so all three are accepted.
func parseSignatureInputs(raw json.RawMessage) ([]signatureInput, error) {
	doc := strings.TrimSpace(string(raw))
	if strings.HasPrefix(doc, `"`) {
		var inner string
		if err := json.Unmarshal(raw, &inner); err != nil {
			return nil, fmt.Errorf("decoding signature string: %w", err)
		}
		doc = inner
	}

	var inputs []signatureInput
	if err := json.Unmarshal([]byte(doc), &inputs); err == nil {
		return inputs, nil
	}

	repaired := strings.NewReplacer(`'`, `"`, "True", "true", "False", "false", "None", "null").Replace(doc)
	if err := json.Unmarshal([]byte(repaired), &inputs); err != nil {
		return nil, fmt.Errorf("decoding signature inputs: %w", err)
	}
	return inputs, nil
}

// featureColumns turns signature inputs into columns. Unnamed inputs
// (a single tensor, for example) have nothing to show as a column name
// and are left out.
func featureColumns(inputs []signatureInput) []pluginsdk.Column {
	columns := make([]pluginsdk.Column, 0, len(inputs))
	for _, in := range inputs {
		if in.Name == "" {
			continue
		}
		columns = append(columns, pluginsdk.Column{
			Name:     in.Name,
			DataType: in.Type,
			Nullable: !in.required(),
		})
	}
	return columns
}
