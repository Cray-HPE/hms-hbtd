@Library('dst-shared@master') _

dockerBuildPipeline {
        githubPushRepo = "Cray-HPE/hms-hmi-service"
        repository = "cray"
        imagePrefix = "cray"
        app = "hbtd"
        name = "hms-hbtd"
        description = "Cray heartbeat tracker service"
        dockerfile = "Dockerfile"
        slackNotification = ["", "", false, false, true, true]
        product = "csm"
}
