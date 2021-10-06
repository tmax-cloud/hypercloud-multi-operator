# HyperCloud-Multi-Operator changelog!!
All notable changes to this project will be documented in this file.

<!-------------------- v5.0.25.2 start -------------------->

## HyperCloud-Multi-Operator_5.0.25.2 (2021. 09. 10. (금) 17:17:27 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc
  - [etc] cluster template update(tsb 0.2.0 호환) by soohwan kim
  - [etc] push.sh 버전 관리 버그 픽스 by soohwan kim
  - [etc] manager에 Seoul region time zone configuration 추가 by soohwan kim

<!--------------------- v5.0.25.2 end --------------------->

<!-------------------- v5.0.25.1 start -------------------->

## HyperCloud-Multi-Operator_5.0.25.1 (2021. 09. 07. (화) 13:25:15 KST)

### Added

### Changed

### Fixed
  - [ims][269425] vsphere cluster claim bug fix by soohwan kim
  - [ims][268314] kubeconfig file업로드를 위한 clusterregistration.spec.kubeconfig.format: data-url 추가 by soohwan kim

### CRD yaml

### Etc
  - [etc] push.sh script update, crd update by soohwan kim
  - [etc] spec.kubeconfig.format: data-url 추가 by GitHub

<!--------------------- v5.0.25.1 end --------------------->

<!-------------------- v5.0.25.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.25.0 (2021. 08. 27. (금) 18:55:57 KST)

### Added
  - [Feat] Cluster Claim Spec하위절 구조 변경 by soohwan kim
  - [feat] vsphere cluster claim기능 추가 by soohwan kim

### Changed
  - [Mod] cluster claim, cluster manager 일부스펙 수정 by soohwan kim

### Fixed
  - [ims][268314] 클러스터 등록시 base64인코딩된 kubeconfig파일을 받아서 secret생성 하도록 로직 수정 by soohwan kim

### CRD yaml

### Etc
  - [etc] 테스트 레포 태그 제거 by soohwan kim
  - [etc] commit_rule.md 추가 by soohwan kim
  - [Etc] capi template file추가 by soohwan kim
  - [Etc] cluster claim property name 에러 수정, 불필요한 로깅 제거 by soohwan kim
  - [Etc] 스펙명 오타 수정 by soohwan kim
  - [etc] cluster manager spec에 provider 복구 by soohwan kim
  - [Etc] 코드 indenent mismatch 수정, Makefile 주석 수정 by soohwan kim

<!--------------------- v5.0.25.0 end --------------------->

<!-------------------- v5.0.24.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.24.0 (2021. 08. 19. (목) 17:02:50 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.24.0 end --------------------->

<!-------------------- v5.0.23.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.23.0 (2021. 08. 12. (목) 13:05:48 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.23.0 end --------------------->

<!-------------------- v5.0.22.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.22.0 (2021. 08. 05. (목) 15:16:23 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.22.0 end --------------------->

<!-------------------- v5.0.21.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.21.0 (2021. 07. 29. (목) 15:33:04 KST)

### Added
  - [feat] cluster template parameter에 namespace가 추가되어 operator에 처리로직 추가 by soohwan kim

### Changed

### Fixed

### CRD yaml

### Etc
  - [etc] image 이름 정책 변경, operator-sdk환경변수 추가 by soohwan kim

<!--------------------- v5.0.21.0 end --------------------->

<!-------------------- v5.0.20.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.20.0 (2021. 07. 22. (목) 14:19:23 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.20.0 end --------------------->

<!-------------------- v5.0.19.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.19.0 (2021. 07. 15. (목) 17:21:19 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc
  - [etc] 불필요한 컨틀로러 제거 by chosangwon93

<!--------------------- v5.0.19.0 end --------------------->

<!-------------------- v5.0.18.1 start -------------------->

## HyperCloud-Multi-Operator_5.0.18.1 (2021. 07. 12. (월) 14:16:11 KST)

### Added

### Changed
  - [mod] proxy rule 생성 시 URL 마지막에 / 붙이도록 추가 by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] 코드 정리 by chosangwon93

<!--------------------- v5.0.18.1 end --------------------->

<!-------------------- v5.0.18.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.18.0 (2021. 07. 09. (금) 09:38:11 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.18.0 end --------------------->

<!-------------------- v5.0.17.2 start -------------------->

## HyperCloud-Multi-Operator_5.0.17.2 (2021. 07. 06. (화) 20:43:15 KST)

### Added

### Changed
  - [mod] clustermanager 삭제시 바로 kubefed unjoin하도록 수정 by chosangwon93

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.17.2 end --------------------->

<!-------------------- v5.0.17.1 start -------------------->

## HyperCloud-Multi-Operator_5.0.17.1 (2021. 07. 05. (월) 15:28:45 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.17.1 end --------------------->

<!-------------------- v5.0.17.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.17.0 (2021. 07. 01. (목) 17:46:14 KST)

### Added
  - [feat] 클러스터 등록기능 추가 by chosangwon93

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.17.0 end --------------------->

<!-------------------- v5.0.16.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.16.0 (2021. 06. 25. (금) 11:12:03 KST)

### Added
  - [feat] HPCD를 통해 생성되지 않은 클러스터를 등록하는 기능 추가 by chosangwon93

### Changed
  - [mod] clustermanager가 존재할 때만 배포된 리소스를 삭제할 수 있도록 수정 by chosangwon93
  - [mod] 등록된 클러스터를 삭제할 때 배포된 role/binding을 제거하도록 수정 by chosangwon93

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.16.0 end --------------------->

<!-------------------- v5.0.15.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.15.0 (2021. 06. 17. (목) 15:10:50 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.15.0 end --------------------->

<!-------------------- v5.0.14.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.14.0 (2021. 06. 10. (목) 20:22:22 KST)

### Added

### Changed
  - [mod] 클러스터가 삭제되었을 때 db에 직접 접근해서 cluster 정보를 지우지 않고 hypercloud를 호출하도록 수정' by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] .gitignore 수정 by chosangwon93

<!--------------------- v5.0.14.0 end --------------------->

<!-------------------- v5.0.13.1 start -------------------->

## HyperCloud-Multi-Operator_5.0.13.1 (2021. 06. 03. (목) 18:56:06 KST)

### Added

### Changed
  - [mod] 불필요한 delete 이벤트에 대한 webhook 로직 삭제 by chosangwon93

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.13.1 end --------------------->

<!-------------------- v5.0.13.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.13.0 (2021. 06. 03. (목) 17:23:20 KST)

### Added

### Changed

### Fixed

### CRD yaml

### Etc

<!--------------------- v5.0.13.0 end --------------------->

<!-------------------- v5.0.12.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.12.0 (2021. 05. 27. (목) 15:05:09 KST)

### Added

### Changed
  - [mod] 네임스페이스 내에서 클러스터 이름이 중복되지 않게 변경되면서 ClusterManager에서 불필요한 필드 제거 및 로직 수정 by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] add key-mapping files by chosangwon93

<!--------------------- v5.0.12.0 end --------------------->

<!-------------------- v5.0.11.4 start -------------------->

## HyperCloud-Multi-Operator_5.0.11.4 (Tue May 25 05:10:36 KST 2021)

### Added

### Changed
  - [mod] 클러스터 클레임 초기 생성 시 업데이트가 안되는 문제 해결 by chosangwon93
  - [mod] 하이퍼 클라우드 초기 설치 시 마스터 클러스터에 대한 프록시 설정이 되지 않는 문제 해결 by chosangwon93
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.11.4 end --------------------->

<!-------------------- v5.0.11.2 start -------------------->

## HyperCloud-Multi-Operator_5.0.11.2 (Tue May 25 03:00:09 KST 2021)

### Added

### Changed
  - [mod] 하이퍼 클라우드 초기 설치 시 마스터 클러스터에 대한 프록시 설정이 되지 않는 문제 해결 by chosangwon93
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.11.2 end --------------------->
-e 
<!-------------------- v5.0.11.1 start -------------------->
-e 
## HyperCloud-Multi-Operator_5.0.11.1 (2021. 05. 21. (금) 17:31:10 KST)
-e 
### Added
-e 
### Changed
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93
-e 
### Fixed
-e 
### CRD yaml
-e 
### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93
-e 
<!--------------------- v5.0.11.1 end --------------------->

<!-------------------- v5.0.11.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.11.0 (Thu May 20 08:21:10 KST 2021)

### Added

### Changed
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.11.0 end --------------------->

<!-------------------- v5.0.10.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.10.0 (Thu May 13 08:41:47 KST 2021)

### Added

### Changed
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.10.0 end --------------------->

<!-------------------- v5.0.9.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.9.0 (Thu May  6 09:36:33 KST 2021)

### Added

### Changed
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.9.0 end --------------------->

<!-------------------- v5.0.8.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.8.0 (Fri Apr 30 08:58:34 KST 2021)

### Added

### Changed
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.8.0 end --------------------->

<!-------------------- v5.0.7.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.7.0 (Thu Apr 22 10:32:21 KST 2021)

### Added

### Changed
  - [mod] delete aws elb by chosangwon93
  - [mod] add resource req, limit by chosangwon93
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.7.0 end --------------------->

<!-------------------- v5.0.6.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.6.0 (Thu Apr  8 09:41:36 KST 2021)

### Added

### Changed
  - [mod] scope by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] delete duplicated manifests directory by chosangwon93
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.6.0 end --------------------->

<!-------------------- v5.0.3.1 start -------------------->

## HyperCloud-Multi-Operator_5.0.3.1 (Mon Apr  5 02:46:02 KST 2021)

### Added

### Changed
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.3.1 end --------------------->

<!-------------------- v5.0.5.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.5.0 (Thu Apr  1 08:55:44 KST 2021)

### Added

### Changed
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.5.0 end --------------------->

<!-------------------- v5.0.4.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.4.0 (Thu Mar 25 09:30:22 KST 2021)

### Added

### Changed
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.4.0 end --------------------->

<!-------------------- v5.0.3.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.3.0 (Thu Mar 18 17:54:55 KST 2021)

### Added

### Changed
  - [mod] remove status.member by chosangwon93
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.3.0 end --------------------->

<!-------------------- v5.0.2.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.2.0 (Thu Mar 11 19:33:34 KST 2021)

### Added

### Changed
  - [mod] remove status.member by chosangwon93
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.2.0 end --------------------->

<!-------------------- v5.0.1.0 start -------------------->

## HyperCloud-Multi-Operator_5.0.1.0 (Mon Mar  8 15:54:52 KST 2021)

### Added

### Changed
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.1.0 end --------------------->

<!-------------------- v5.0.0.43 start -------------------->

## HyperCloud-Multi-Operator_5.0.0.43 (Mon Mar  8 15:40:09 KST 2021)

### Added

### Changed
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.0.43 end --------------------->

<!-------------------- v5.0.0.42 start -------------------->

## HyperCloud-Multi-Operator_5.0.0.42 (Mon Mar  8 15:33:38 KST 2021)

### Added

### Changed
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.0.42 end --------------------->

<!-------------------- v5.0.0.40 start -------------------->

## HyperCloud-Multi-Operator_5.0.0.40 (Mon Mar  8 15:25:58 KST 2021)

### Added

### Changed
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.0.40 end --------------------->

<!-------------------- v5.0.0.2 start -------------------->

## HyperCloud-Multi-Operator_5.0.0.2 (Tue Mar  2 13:29:05 KST 2021)

### Added

### Changed
  - [mod] add serviceinstance scheme by chosangwon93

### Fixed

### CRD yaml

### Etc
  - [etc] change webhook-server-cert name by chosangwon93
  - [etc] pkg update by chosangwon93
  - [etc] init by chosangwon93

<!--------------------- v5.0.0.2 end --------------------->

<!-------------------- v5.0.0.1 start -------------------->

## HyperCloud-Multi-Operator_5.0.0.1 (Thu Feb 25 13:15:30 KST 2021)

### Added

### Changed

### Fixed

### CRD yaml

### Etc
  - [etc] init by chosangwon93

<!--------------------- v5.0.0.1 end --------------------->
