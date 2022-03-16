package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"omni-manager/models"
	"omni-manager/util"
	"strconv"

	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// @Summary StartBuild Job
// @Description start a image build job
// @Tags  meta Manager
// @Param	body		body 	models.ImageInputData	true		"body for Metadata content"
// @Accept json
// @Produce json
// @Router /images/startBuild [post]
func StartBuild(c *gin.Context) {
	var imageInputData models.ImageInputData
	err := c.ShouldBindJSON(&imageInputData)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, err, nil))
		return
	}
	var insertData models.Metadata
	insertData.Packages = imageInputData.Packages
	insertData.Version = imageInputData.Version
	insertData.BuildType = imageInputData.BuildType
	if len(insertData.Version) == 0 {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, "verison not allowed empty ", nil))
		return
	}
	valide := false
	for _, pkgs := range util.GetConfig().BuildParam.Packages {
		if pkgs == insertData.Packages {
			valide = true
			break
		}
	}
	if !valide {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, "packages not supported  ", util.GetConfig().BuildParam.Packages))
		return
	}
	valide = false
	for _, buildtype := range util.GetConfig().BuildParam.BuildType {
		if buildtype == insertData.BuildType {
			valide = true
			break
		}
	}
	if !valide {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, "buildType not supported  ", util.GetConfig().BuildParam.BuildType))
		return
	}

	var temp []byte
	temp, err = json.Marshal(imageInputData.CustomPkg)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, err, nil))
		return
	}
	insertData.CustomPkg = string(temp)
	//----------------------send data to k8s to build----
	var jobID = fmt.Sprintf(`omni-image-%s`, uuid.NewV4().String())
	var imageName = fmt.Sprintf(`openEuler-%s.iso`, uuid.NewV4().String())
	controllerID := uuid.NewV4().String()

	omniImager := `omni-imager --package-list /etc/omni-imager/` + insertData.Packages + `.json --config-file /etc/omni-imager/conf.yaml --build-type ` + insertData.BuildType + ` --output-file ` + imageName
	omniCurl := `curl -vvv -Ffile=@/opt/omni-workspace/` + imageName + ` -Fproject=openeuler20.03  -FfileType=image 'https://repo.test.osinfra.cn/data/upload?token=316462d0c029ba707ad2'`
	deployment := &appsv1.Deployment{

		ObjectMeta: metav1.ObjectMeta{
			Name:      jobID,
			Namespace: metav1.NamespaceDefault,
		},
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"job-name": jobID, "controller-uid": controllerID},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"job-name": jobID, "controller-uid": controllerID},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:            "image-builder",
							Image:           "tommylike/omni-worker:0.0.1",
							SecurityContext: &(apiv1.SecurityContext{Privileged: models.BoolPtr(true)}),
							Command:         []string{"/bin/sh", "-c", omniImager, omniCurl},
						},
					},
					RestartPolicy: "Always", //Never not supported .
				},
			},
		},
	}

	deploy, err := models.GetClientSet().AppsV1().Deployments(metav1.NamespaceDefault).Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusServerError, "Create job Error", err))
		return
	}
	insertData.ContainerName = deploy.ObjectMeta.Name
	insertData.CreateTime = deploy.ObjectMeta.CreationTimestamp.Time
	_, err = models.AddMetadata(&insertData)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusServerError, err, nil))
		return
	}
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, nil, deploy.ObjectMeta.Name))
}

// @Summary QueryJobStatus
// @Description QueryJobStatus for given job name
// @Tags  meta Manager
// @Param	name		path 	string	true		"The name for job"
// @Accept json
// @Produce json
// @Router /images/queryJobStatus/{name} [get]
func QueryJobStatus(c *gin.Context) {
	name := c.Param("name")
	jobAPI := models.GetClientSet().BatchV1()
	job, err := jobAPI.Jobs(metav1.NamespaceDefault).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusServerError, " QueryJobStatus Error:", err))
		return
	}
	completions := job.Spec.Completions
	backoffLimit := job.Spec.BackoffLimit
	const JOB_STATUS_RUNNING = "running"
	const JOB_STATUS_SUCCEED = "succeed"
	const JOB_STATUS_FAILED = "failed"
	result := make(map[string]interface{})
	result["name"] = name
	result["startTime"] = job.Status.StartTime

	// check status
	if job.Status.Succeeded > *completions {
		result["status"] = JOB_STATUS_SUCCEED
		result["completionTime"] = job.Status.CompletionTime
	} else if job.Status.Failed > *backoffLimit {
		result["status"] = JOB_STATUS_FAILED
		result["completionTime"] = job.Status.CompletionTime
	} else if job.Status.Succeeded == 0 || job.Status.Failed == 0 {
		result["status"] = JOB_STATUS_RUNNING
	}
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, "ok", result))
}

// @Summary get
// @Description get single one
// @Tags  meta Manager
// @Param	id		path 	string	true		"The key for staticblock"
// @Accept json
// @Produce json
// @Router /images/get/{id} [get]
func Read(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if id <= 0 || err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, "id must be int type", err))
		return
	}

	v, err := models.GetMetadataById(id)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusServerError, err, nil))
		return
	}
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, id, v))
}

// @Summary GetBaseData param
// @Description get architecture, release Version, output Format ,and default package name list
// @Tags  meta Manager
// @Accept json
// @Produce json
// @Router /images/param/getBaseData/ [get]
func GetBaseData(c *gin.Context) {
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, "ok", util.GetConfig().BuildParam, util.GetConfig().DefaultPkgList))
}

// @Summary GetCustomePkgList param
// @Description get default package name list. this list load from https://raw.githubusercontent.com/omnibuildplatform/omni-imager/main/etc/openEuler-minimal.json
// @Tags  meta Manager
// @Accept json
// @Produce json
// @Router /images/param/getCustomePkgList/ [get]
func GetCustomePkgList(c *gin.Context) {
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, "ok", nil))
}

// @Summary query multi datas
// @Description use param to query multi datas
// @Tags  meta Manager
// @Param	project_name		query 	string	true		"project name"
// @Param	pkg_name		query 	string	true		"package name"
// @Accept json
// @Produce json
// @Router /images/query [get]
func Query(c *gin.Context) {
	//...... emplty . wait for query param
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, c.Query("project_name"), c.Query("pkg_name")))
}

// @Summary update
// @Description update single data
// @Tags  meta Manager
// @Param	body		body 	models.ImageInputData	true		"body for Metadata content"
// @Accept json
// @Produce json
// @Router /images/update [put]
func Update(c *gin.Context) {
	var imageInputData models.ImageInputData
	err := c.ShouldBindJSON(&imageInputData)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, err, nil))
		return
	}
	var updateData models.Metadata
	updateData.Packages = imageInputData.Packages
	updateData.Version = imageInputData.Version
	updateData.BuildType = imageInputData.BuildType
	var temp []byte
	// temp, err = json.Marshal(imageInputData.BasePkg)
	// if err != nil {
	// 	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, err, nil))
	// 	return
	// }
	// updateData.BasePkg = string(temp)
	temp, err = json.Marshal(imageInputData.CustomPkg)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, err, nil))
		return
	}
	updateData.CustomPkg = string(temp)
	//use origin item id
	updateData.Id = imageInputData.Id

	err = models.UpdateMetadataById(&updateData)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusServerError, err, nil))
		return
	}
	util.Log.Warnf("The MetaData of Id (%d) had been update to: %s", updateData.Id, updateData.ToString())
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, "ok", nil))
}

// @Summary delete
// @Description update single data
// @Tags  meta Manager
// @Param	id		path 	string	true		"The key for staticblock"
// @Accept json
// @Produce json
// @Router /images/delete/:id [delete]
func Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if id <= 0 || err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusClientError, "id must be int type", err))
		return
	}
	err = models.DeleteMetadata(id)
	if err != nil {
		c.JSON(http.StatusOK, util.ExportData(util.CodeStatusServerError, err, nil))
		return
	}
	util.Log.Warnf("The  MetaData (Id:%d) had been delete ", id)
	c.JSON(http.StatusOK, util.ExportData(util.CodeStatusNormal, "ok", id))
}
