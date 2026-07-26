pipeline {
    agent {
        label 'linux && ephemeral && untrusted'
    }

    options {
        buildDiscarder(logRotator(numToKeepStr: '20'))
        disableConcurrentBuilds(abortPrevious: true)
        skipDefaultCheckout(true)
        timeout(time: 75, unit: 'MINUTES')
        timestamps()
    }

    environment {
        CI = 'true'
        PYTHONDONTWRITEBYTECODE = '1'
        PRESUBMIT_REQUIRE_PR_CHECKLIST = 'false'
    }

    stages {
        stage('Checkout') {
            steps {
                deleteDir()
                checkout scm
            }
        }

        stage('PS0') {
            steps {
                sh 'python3 scripts/ci/presubmit.py PS0'
            }
        }

        stage('PS1') {
            steps {
                sh 'python3 scripts/ci/presubmit.py PS1'
            }
        }

        stage('PS2') {
            when {
                anyOf {
                    changeRequest target: 'main'
                    branch 'main'
                }
            }
            steps {
                sh 'python3 scripts/ci/presubmit.py PS2'
            }
        }
    }

    post {
        always {
            deleteDir()
        }
    }
}
