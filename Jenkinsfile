@Library('dst-shared@master') _

dockerBuildPipeline {
        repository = "cray"
        imagePrefix = "cray"
        app = "hbtd"
        name = "hms-hbtd"
        description = "Cray heartbeat tracker service"
        dockerfile = "Dockerfile"
        slackNotification = ["", "", false, false, true, true]
        product = "csm"
}
