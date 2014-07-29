package staging_messages

import "github.com/cloudfoundry-incubator/runtime-schema/models"

type StagingRequestFromCC struct {
	AppId                          string      `json:"app_id"`
	TaskId                         string      `json:"task_id"`
	Stack                          string      `json:"stack"`
	AppBitsDownloadUri             string      `json:"app_bits_download_uri"`
	BuildArtifactsCacheDownloadUri string      `json:"build_artifacts_cache_download_uri,omitempty"`
	FileDescriptors                int         `json:"file_descriptors"`
	MemoryMB                       int         `json:"memory_mb"`
	DiskMB                         int         `json:"disk_mb"`
	Buildpacks                     []Buildpack `json:"buildpacks"`
	Environment                    Environment `json:"environment"`
}

type Buildpack struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	Url  string `json:"url"`
}

type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Environment []EnvironmentVariable

func (env Environment) BBSEnvironment() []models.EnvironmentVariable {
	bbsEnv := make([]models.EnvironmentVariable, len(env))
	for i, envVar := range env {
		bbsEnv[i] = models.EnvironmentVariable{Name: envVar.Name, Value: envVar.Value}
	}
	return bbsEnv
}

type StagingResponseForCC struct {
	AppId                string `json:"app_id,omitempty"`
	TaskId               string `json:"task_id,omitempty"`
	BuildpackKey         string `json:"buildpack_key,omitempty"`
	DetectedBuildpack    string `json:"detected_buildpack,omitempty"`
	DetectedStartCommand string `json:"detected_start_command,omitempty"`
	Error                string `json:"error,omitempty"`
}