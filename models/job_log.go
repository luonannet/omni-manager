package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/omnibuildplatform/omni-manager/util"

	uuid "github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	BuildImageFromRelease string = "buildimagefromrelease"
	BuildImageFromISO     string = "buildimagefromiso"
)

//post this body to backend
type BuildParam struct {
	// Id        int      `gorm:"primaryKey"`
	Arch      string   ` description:"architecture"`
	Release   string   ` description:"release openEuler Version"`
	BuildType string   ` description:"iso , zip ...."`
	CustomPkg []string ` description:"custom"`
	Label     string   ` description:"name"`
	Desc      string   ` description:"description"`
}
type JobLog struct {
	JobName          string    ` description:"pod name" gorm:"primaryKey"`
	Arch             string    ` description:"architecture"`
	Release          string    ` description:"release openEuler Version"`
	BuildType        string    ` description:"iso , zip ...."`
	BasePkg          string    ` gorm:"size:5055"  description:"default package"`
	CustomPkg        string    ` gorm:"size:5055" description:"custom"`
	UserId           int       ` description:"user id"  gorm:"index:"`
	UserName         string    ` description:"user name"`
	CreateTime       time.Time ` description:"create time"`
	Status           string    ` description:"current status :running ,success, failed" gorm:"index"`
	DownloadUrl      string    ` description:"download the result of build iso file"`
	ConfigMapName    string    ` description:"configMap name"`
	JobLabel         string    ` description:"job label"`
	JobDesc          string    ` description:"job description"`
	StartTime        time.Time ` description:"create time"`
	EndTime          time.Time ` description:"create time"`
	JobType          string    ` description:"job type"`
	KickStartID      string    ` description:"KickStart id"  `
	BaseImageID      string    ` description:"BaseImage id"  `
	KickStartContent string    ` description:"kick_start_content"  `
}
type SummaryStatus struct {
	Succeed int `json:"succeed"`
	Running int `json:"running"`
	Failed  int `json:"failed"`
	Created int `json:"created"`
	Stopped int `json:"stopped"`
}

type JobStatuItem struct {
	Id        string `json:"id"`
	State     string `json:"state"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type BaseImageConfig struct {
	Id        string `json:"id"`
	State     string `json:"state"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

func (t *JobLog) TableName() string {
	return "job_log"
}

func (t *JobLog) ToString() string {
	return fmt.Sprintf(" Architecture:%s;EulerVersion:%s;OutFormat:%s;UserId:%d;UserName:%s;JobName:%s", t.Arch, t.Release, t.BuildType, t.UserId, t.UserName, t.JobName)
}

// AddJobLog insert a new ImageMeta into database and returns
// last inserted Id on success.
func AddJobLog(m *JobLog) (err error) {
	o := util.GetDB()
	result := o.Create(m)
	return result.Error
}

func GetJobLogByJobName(jobname string) (v *JobLog, err error) {
	o := util.GetDB()
	v = new(JobLog)
	sql := fmt.Sprintf("select * from %s where job_name = '%s' order by create_time desc limit 1", v.TableName(), jobname)
	tx := o.Raw(sql).Scan(v)
	return v, tx.Error
}

// GetAllJobLog retrieves all ImageMeta matches certain condition. Returns empty list if
// no records exist
func GetAllJobLog(query map[string]string, fields []string, sortby []string, order []string,
	offset int64, limit int64) (ml []interface{}, err error) {
	return nil, err
}

// GetMyJobLogs query my build history
func GetMyJobLogs(jobitem *JobLog, nameOrDesc string, offset int, limit int) (total int64, ml []*JobLog, err error) {
	o := util.GetDB()
	tx := o.Model(jobitem)
	if len(jobitem.Arch) > 0 {
		tx = tx.Where("arch = ?", jobitem.Arch)
	}
	if len(jobitem.Status) > 0 {
		tx = tx.Where("status = ?", jobitem.Status)
	}
	if len(jobitem.BuildType) > 0 {
		tx = tx.Where("build_type = ?", jobitem.BuildType)
	}

	tx = tx.Where("user_id = ?", jobitem.UserId)

	if len(nameOrDesc) > 0 {
		tx = tx.Where("job_label like '%" + nameOrDesc + "%'  or job_desc like '%" + nameOrDesc + "%' ")
	}
	tx.Count(&total)
	tx.Limit(limit).Offset(offset).Order("create_time desc").Scan(&ml)
	return total, ml, tx.Error
}

// DeleteJobLogById
func DeleteJobLogById(jobName string) (err error) {
	o := util.GetDB()
	m := new(JobLog)
	m.JobName = jobName
	result := o.Delete(m)
	return result.Error
}

// DeleteMultiJobLogs
func DeleteMultiJobLogs(names string) (err error) {
	o := util.GetDB()
	m := new(JobLog)
	sql := fmt.Sprintf("delete from %s  where job_name in (%s)", m.TableName(), names)
	result := o.Model(m).Exec(sql)
	return result.Error
}

// UpdateJobLogStatusById
func UpdateJobLogStatusById(jobname, newStatus string) (err error) {
	o := util.GetDB()
	m := new(JobLog)
	sql := fmt.Sprintf("update %s set status='%s' where job_name = '%s'", m.TableName(), newStatus, jobname)
	result := o.Model(m).Exec(sql)
	return result.Error
}

// CountSummaryStatus
func CountSummaryStatus(userid int) (result *SummaryStatus, err error) {
	o := util.GetDB()
	m := new(JobLog)
	sql := fmt.Sprintf("select  count(case when status ='running' then '1' end) as 'running', count(case when status ='failed' then '1' end) as 'failed', count(case when status ='succeed' then '1' end) as 'succeed' ,  count(case when status ='created' then '1' end) as 'created',  count(case when status ='stopped' then '1' end) as 'stopped'  FROM %s where user_id = %d ", m.TableName(), userid)
	result = new(SummaryStatus)
	tx := o.Raw(sql).Scan(result)
	return result, tx.Error
}

// CreateTables
func CreateTables() (err error) {
	o := util.GetDB()
	o.Migrator().AutoMigrate(&JobLog{})
	o.Migrator().AutoMigrate(&BaseImages{})
	o.Migrator().AutoMigrate(&KickStart{})
	return
}

//Persistence a Job_log  from redis to db
func PersistenceJob(m *JobLog) (err error) {
	err = util.DelKey(CreateRedisJobName(m.JobName), nil)
	if err != nil {
		return
	}
	return AddJobLog(m)

}

//make ConfigMap
func MakeConfigMap(release string, customRpms []string) (cm *v1.ConfigMap) {
	totalPkgs := make(map[string][]string)
	totalPkgs["packages"] = append(util.GetConfig().DefaultPkgList.Packages, customRpms...)
	confYmalConentBytes, err := json.Marshal(totalPkgs)
	if err != nil {
		return
	}

	configMapName := fmt.Sprintf("cmname%d", time.Now().In(util.CnTime).Unix())
	tempdata := make(map[string]string)
	tempdata["working_dir"] = "/opt/omni-workspace"
	tempdata["debug"] = "True"
	tempdata["user_name"] = "root"
	tempdata["user_passwd"] = "openEuler"
	tempdata["installer_configs"] = "/etc/omni-imager/installer_assets/calamares-configs"
	tempdata["systemd_configs"] = "/etc/omni-imager/installer_assets/systemd-configs"
	tempdata["init_script"] = "/etc/omni-imager/init"
	tempdata["installer_script"] = "/etc/omni-imager/runinstaller"
	tempdata["repo_file"] = fmt.Sprintf("/etc/omni-imager/repos/%s.repo", release)
	tempdata["use_cached_rootfs"] = "True"
	tempdata["cached_rootfs_gz"] = "/opt/rootfs_cache/rootfs.tar.gz"

	tempdataBytes, _ := yaml.Marshal(tempdata)

	configMapType := metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}
	var configImage *v1.ConfigMap
	configImage = &v1.ConfigMap{
		TypeMeta: configMapType,
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: util.GetConfig().K8sConfig.Namespace,
		},
		Data: map[string]string{
			"conf.yaml":      string(tempdataBytes),
			"totalrpms.json": string(confYmalConentBytes),
		},
	}
	cm, err = clientset.CoreV1().ConfigMaps(util.GetConfig().K8sConfig.Namespace).Create(context.TODO(), configImage, metav1.CreateOptions{
		TypeMeta: configImage.TypeMeta,
	})
	if err != nil {
		return
	}
	cm.TypeMeta = configMapType
	cm.Name = configMapName

	return
}

//make job yaml and start job
func MakeJob(cm *v1.ConfigMap, buildtype, release string) (job *batchv1.Job, err error) {
	controllerID := uuid.NewV4().String()
	var jobName = fmt.Sprintf(`omni-image-%s`, controllerID)
	outputName := fmt.Sprintf(`openEuler-%s.iso`, controllerID)
	clientset, err := kubernetes.NewForConfig(GetK8sConfig())
	if err != nil {
		return
	}
	omniImager := `omni-imager --package-list /conf/totalrpms.json --config-file /conf/conf.yaml --build-type ` + buildtype + ` --output-file ` + outputName + ` && curl -vvv -Ffile=@/opt/omni-workspace/` + outputName + ` -Fproject=` + release + `  -FfileType=image '` + util.GetConfig().BuildServer.OmniRepoAPIInternal + `/images/upload'`
	jobInterface := clientset.BatchV1().Jobs(util.GetConfig().K8sConfig.Namespace)
	var backOffLimit int32 = 0
	var tTLSecondsAfterFinished int32 = 1800
	var privileged bool = true
	var ownerReferenceController bool = false
	var BlockOwnerDeletion bool = true
	jobYaml := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: util.GetConfig().K8sConfig.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         cm.APIVersion,
					Kind:               cm.Kind,
					Name:               cm.Name,
					Controller:         &ownerReferenceController,
					UID:                cm.UID,
					BlockOwnerDeletion: &BlockOwnerDeletion,
				},
			},
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  jobName,
							Image: util.GetConfig().K8sConfig.Image,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							Command: []string{
								"/bin/sh",
								"-c",
								omniImager,
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "confyaml",
									MountPath: "/conf",
								},
								{
									Name:      "rootfs",
									MountPath: "/opt/rootfs_cache",
								},
							},
							// Lifecycle: &v1.Lifecycle{
							// 	PreStop: &v1.LifecycleHandler{
							// 		Exec: &v1.ExecAction{
							// 			Command: []string{},
							// 		},
							// 	},
							// },
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
					Volumes: []v1.Volume{
						{
							Name: "confyaml",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: cm.Name,
									},
								},
							},
						},
						{
							Name: "rootfs",
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: "cce-sfs-rootfs",
								},
							},
						},
					},
				},
			},
			BackoffLimit:            &backOffLimit,
			TTLSecondsAfterFinished: &tTLSecondsAfterFinished,
		},
	}
	job, err = jobInterface.Create(context.TODO(), jobYaml, metav1.CreateOptions{})

	return
}
func CreateRedisJobName(jobname string) string {
	return fmt.Sprintf("build_log:%s", jobname)
}
func CheckPodStatus(ns, jobname string) (result map[string]interface{}, job *batchv1.Job, err error) {
	jobAPI := GetClientSet().BatchV1()
	// var job *batchv1.Job
	job, err = jobAPI.Jobs(ns).Get(context.TODO(), jobname, metav1.GetOptions{})
	if err != nil {
		util.Log.Errorf("CheckPodStatus Error:%s", err)
		return
	}

	var JobLog *JobLog
	JobLog, err = GetJobLogByJobName(jobname)
	completions := job.Spec.Completions
	backoffLimit := job.Spec.BackoffLimit
	result = make(map[string]interface{})
	result["name"] = jobname
	result["startTime"] = job.Status.StartTime
	// check status
	if job.Status.Succeeded >= *completions {
		result["status"] = JOB_STATUS_SUCCEED
		result["completionTime"] = job.Status.CompletionTime
		if JobLog != nil {
			result["url"] = JobLog.DownloadUrl
			UpdateJobLogStatusById(jobname, JOB_STATUS_SUCCEED)
		}
		job = nil
	} else if job.Status.Failed > *backoffLimit {
		result["status"] = JOB_STATUS_FAILED
		result["error"] = job.Status.String()
		result["completionTime"] = job.Status.CompletionTime
		UpdateJobLogStatusById(jobname, JOB_STATUS_FAILED)
	} else if job.Status.Succeeded == 0 || job.Status.Failed == 0 {
		result["status"] = JOB_STATUS_RUNNING
	}
	return
}

type jobNameType struct {
	JobName string `json:"job_name"`
	JobType string `json:"job_type"`
}

func SyncJobStatus() {
	m := new(JobLog)
	sql := fmt.Sprintf("select job_name,job_type from %s where status not in ('%s','%s','%s')", m.TableName(), JOB_STATUS_SUCCEED, JOB_STATUS_FAILED, JOB_STATUS_STOPPED)
	var jobIdList []jobNameType
	param := make(map[string]interface{})
	param["service"] = "omni"
	param["domain"] = "omni-build"

	o := util.GetDB()
	for {
		jobIdList = make([]jobNameType, 0)
		o.Raw(sql).Scan(&jobIdList)
		if len(jobIdList) == 0 {
			time.Sleep(time.Second * 60)
			continue
		}

		var releaseList []string
		var isoList []string
		for _, item := range jobIdList {
			if item.JobType == BuildImageFromISO {
				isoList = append(isoList, item.JobName)
			} else if item.JobType == BuildImageFromRelease {
				releaseList = append(releaseList, item.JobName)
			}
		}
		var totalJobStatusList []JobStatuItem
		step := 0
	nextType:
		step++
		var paramBytes []byte
		if len(isoList) > 0 {
			param["IDs"] = isoList
			param["task"] = BuildImageFromISO
		} else if len(releaseList) > 0 {
			param["IDs"] = releaseList
			param["task"] = BuildImageFromRelease
		}
		paramBytes, _ = json.Marshal(param)
		var req *http.Request
		var err error
		req, err = http.NewRequest("POST", util.GetConfig().BuildServer.ApiUrl+"/v1/jobs/batchQuery", strings.NewReader(string(paramBytes)))
		if err != nil {
			util.Log.Errorln("title:SyncJobStatus,NewRequest:" + err.Error())
			time.Sleep(time.Second * 30)
			continue
		}
		resp, err := http.DefaultClient.Do(req) //http.Get(url)
		if err != nil {
			util.Log.Errorln("title:SyncJobStatus,reason:" + err.Error())
			time.Sleep(time.Second * 30)
			continue
		}

		resultBytes, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			util.Log.Errorln("title:SyncJobStatus,ReadAll:" + string(resultBytes))
			time.Sleep(time.Second * 30)
			continue
		}
		resp.Body.Close()
		var jobStatusList []JobStatuItem
		err = json.Unmarshal(resultBytes, &jobStatusList)
		if err != nil {
			util.Log.Errorln("title:SyncJobStatus Unmarshal Error,reason:" + err.Error())
			time.Sleep(time.Second * 30)
			continue
		}
		totalJobStatusList = append(totalJobStatusList, jobStatusList...)
		if step == 1 {
			isoList = nil
			goto nextType
		}
		if len(totalJobStatusList) == 0 {
			time.Sleep(time.Second * 30)
			continue
		}
		statuSql := ""
		starttimeSql := ""
		endtimeSql := ""
		dateformatStr := "%Y-%m-%dT%H:%i:%s"
		downloadSql := ""
		ids := ""
		for _, jobStatus := range totalJobStatusList {
			jobStatus.StartTime = string([]byte(jobStatus.StartTime)[:19])
			jobStatus.EndTime = string([]byte(jobStatus.EndTime)[:19])

			switch jobStatus.State {
			case JOB_BUILD_STATUS_FAILED:
				statuSql = statuSql + fmt.Sprintf(" WHEN job_name = '%s' THEN  '%s' ", jobStatus.Id, JOB_STATUS_FAILED)
			case JOB_BUILD_STATUS_SUCCEED:
				statuSql = statuSql + fmt.Sprintf(" WHEN job_name = '%s' THEN   '%s' ", jobStatus.Id, JOB_STATUS_SUCCEED)
				downloadURL := util.GetConfig().BuildServer.OmniRepoAPI + "/images/query?externalID=" + jobStatus.Id
				downloadSql = downloadSql + fmt.Sprintf(" WHEN job_name = '%s' THEN  '%s' ", jobStatus.Id, downloadURL)
			case JOB_BUILD_STATUS_CREATED:
				statuSql = statuSql + fmt.Sprintf(" WHEN job_name = '%s' THEN   '%s' ", jobStatus.Id, JOB_STATUS_CREATED)
			case JOB_BUILD_STATUS_STOPPED:
				statuSql = statuSql + fmt.Sprintf(" WHEN job_name = '%s' THEN   '%s' ", jobStatus.Id, JOB_STATUS_STOPPED)
			case JOB_BUILD_STATUS_RUNNING:
				statuSql = statuSql + fmt.Sprintf(" WHEN job_name = '%s' THEN   '%s' ", jobStatus.Id, JOB_STATUS_RUNNING)
			}
			starttimeSql = starttimeSql + fmt.Sprintf(" WHEN job_name = '%s' THEN  STR_TO_DATE('%v','%s') ", jobStatus.Id, jobStatus.StartTime, dateformatStr)
			endtimeSql = endtimeSql + fmt.Sprintf(" WHEN job_name = '%s' THEN  STR_TO_DATE('%v','%s') ", jobStatus.Id, jobStatus.EndTime, dateformatStr)

			if ids == "" {
				ids = "'" + jobStatus.Id + "'"
			} else {
				ids = ids + ",'" + jobStatus.Id + "'"
			}
		}

		if len(statuSql) == 0 {
			time.Sleep(time.Second * 30)
			continue
		}
		if len(downloadSql) > 0 {
			downloadSql = " end, download_url =case " + downloadSql
		}
		updateSql := (" UPDATE " + m.TableName() + "  SET status = case " + statuSql + "  end  , start_time = case " + starttimeSql + " end, end_time =case " + endtimeSql + downloadSql + "  END where job_name in (" + ids + ");")
		tx := o.Exec(updateSql)
		if tx.Error != nil {
			util.Log.Errorln("title:UPDATE sync Error,reason:" + err.Error())
		}

		time.Sleep(time.Second * 30)
	}

}

type miniBaseImage struct {
	ID   int    ` description:"id" gorm:"primaryKey"`
	Name string ` description:"name"`
}
type miniKickstart struct {
	ID   int    ` description:"id" gorm:"primaryKey"`
	Name string ` description:"name"`
}

func GetImagesAndKickStart(userid int) (result map[string]interface{}, err error) {
	baseImages := new(BaseImages)
	kickStart := new(KickStart)
	result = make(map[string]interface{})
	baseImagesList := make([]*miniBaseImage, 0)
	kickStartList := make([]*miniKickstart, 0)
	o := util.GetDB()
	sql := fmt.Sprintf("select name,id from %s where  user_id = %d and status='%s'", baseImages.TableName(), userid, ImageStatusDone)
	tx := o.Raw(sql).Find(&baseImagesList)
	if tx.Error != nil {
		return nil, tx.Error
	}
	sql = fmt.Sprintf("select name,id from %s where  user_id = %d ", kickStart.TableName(), userid)
	tx = o.Raw(sql).Find(&kickStartList)
	if tx.Error != nil {
		return nil, tx.Error
	}
	result["images"] = baseImagesList
	result["kickstart"] = kickStartList

	return
}
