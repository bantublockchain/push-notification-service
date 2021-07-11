pipeline {
  agent any
  environment {
    registry = 'bantufoundation/push-notification-service'
    registryCredential = 'dockerhub-bantufoundation'
    CAPROVER_CREDENTIALS = credentials("caprover-bantupay")
  }

  stages {
    

    stage('Building image') {
        when {
                expression { BRANCH_NAME ==~ /(alpha|beta|prod|loadtest|wip)/ }
            }
      steps {
        script {
          dockerImage = docker.build("$registry-$BRANCH_NAME:$BUILD_NUMBER", " .")
        }

      }
    }

    stage('Deploy Image') {

         when {
                expression { BRANCH_NAME ==~ /(alpha|beta|prod|loadtest|wip)/ }
            }
            
      steps {
        script {
          docker.withRegistry( '', registryCredential ) {
            dockerImage.push()
            dockerImage.push('latest')
          }
        }

      }
    }

    stage('Remove Unused docker image') {
         when {
                expression { BRANCH_NAME ==~ /(alpha|beta|prod|loadtest|wip)/ }
            }
      steps {
         sh "docker rmi --force `docker images '$registry-$BRANCH_NAME' -a -q`"
      }
    }



    stage('caprover-alpha') {

        when {
                branch 'alpha'
            }

      steps {
        sh '''caprover deploy -h $CAPROVER_CREDENTIALS_USR -p $CAPROVER_CREDENTIALS_PSW -i $registry-$BRANCH_NAME -a pns-alpha
'''
      }
    }

    stage('caprover-prod2') {

        when {
                branch 'loadtest'
            }

      steps {
        sh '''caprover deploy -h $CAPROVER_CREDENTIALS_USR -p $CAPROVER_CREDENTIALS_PSW -i $registry-$BRANCH_NAME -a pns-prod2
'''
      }
    }



    stage('caprover-prod') {

        when {
                branch 'prod'
            }

      steps {
        sh '''caprover deploy -h $CAPROVER_CREDENTIALS_USR -p $CAPROVER_CREDENTIALS_PSW -i $registry-$BRANCH_NAME -a pns-prod
'''
      }
    }

    stage('caprover-wip') {

        when {
                branch 'wip'
            }

      steps {
        sh '''caprover deploy -h $CAPROVER_CREDENTIALS_USR -p $CAPROVER_CREDENTIALS_PSW -i $registry-$BRANCH_NAME -a pns-wip
'''
      }
    }

  }
  
}