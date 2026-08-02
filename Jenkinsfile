pipeline {
    agent none

    options {
        buildDiscarder(logRotator(numToKeepStr: '20'))
        disableConcurrentBuilds(abortPrevious: true)
        skipDefaultCheckout(true)
        timeout(time: 120, unit: 'MINUTES')
        timestamps()
    }

    environment {
        CI = 'true'
        PYTHONDONTWRITEBYTECODE = '1'
        PRESUBMIT_REQUIRE_PR_CHECKLIST = 'false'
        PRESUBMIT_BASE_REF = 'origin/main'
    }

    stages {
        stage('PS0') {
            agent {
                label 'jenkins-verify'
            }
            steps {
                deleteDir()
                retry(3) {
                    checkout scm
                }
                sh 'test "$(git rev-parse HEAD)" = "$EXPECTED_COMMIT"'
                retry(3) {
                    sh 'git fetch --no-tags --unshallow origin +refs/heads/main:refs/remotes/origin/main'
                }
                sh 'python3 scripts/ci/presubmit.py PS0'
            }
            post {
                always {
                    deleteDir()
                }
            }
        }

        stage('PS1') {
            agent {
                label 'jenkins-verify'
            }
            steps {
                deleteDir()
                retry(3) {
                    checkout scm
                }
                sh 'test "$(git rev-parse HEAD)" = "$EXPECTED_COMMIT"'
                sh 'python3 scripts/ci/presubmit.py PS1'
            }
            post {
                always {
                    deleteDir()
                }
            }
        }

        stage('PS2') {
            agent {
                label 'jenkins-integration'
            }
            steps {
                deleteDir()
                retry(3) {
                    checkout scm
                }
                sh 'test "$(git rev-parse HEAD)" = "$EXPECTED_COMMIT"'
                sh 'scripts/ci/wait-for-docker.sh'
                sh 'python3 scripts/ci/presubmit.py PS2'
            }
            post {
                always {
                    archiveArtifacts artifacts: 'artifacts/kafka-scale/**', allowEmptyArchive: true
                    deleteDir()
                }
            }
        }
    }
}
