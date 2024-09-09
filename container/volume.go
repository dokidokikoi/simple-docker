package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	zaplog "github.com/dokidokikoi/go-common/log/zap"
	"go.uber.org/zap"
)

// Create a AUFS filesystem as container root workspace
func NewWorkSpace(volume, imageName, containerName string) {
	CreateReadOnlyLayer(imageName)
	CreateWriteLayer(containerName)
	CreateMountPoint(imageName, containerName)

	// 根据 volume 判断是否执行挂载数据卷操作
	if volume != "" {
		volumeURLs := volumeUrlExtract(volume)
		if len(volumeURLs) == 2 && volumeURLs[0] != "" && volumeURLs[1] != "" {
			MountVolume(volumeURLs, containerName)
		} else {
			zaplog.L().Error("volume parameter input is not correnct.", zap.String("volume", volume))
		}
	}
}

// 解压 tar 格式的镜像文件作只读层
func CreateReadOnlyLayer(imageName string) error {
	unTarFolderUrl := filepath.Join(RootUrl, imageName)
	imageUrl := filepath.Join(RootUrl, imageName+".tar")
	exist, err := PathExists(unTarFolderUrl)
	if err != nil {
		zaplog.L().Sugar().Errorf("fail to judge whether dir %s exist. error: %v", unTarFolderUrl, err)
		return err
	}
	if !exist {
		if err := os.Mkdir(unTarFolderUrl, 0777); err != nil {
			zaplog.L().Sugar().Errorf("mkdir %s fail. error: %v", unTarFolderUrl, err)
			return err
		}
		if _, err := exec.Command("tar", "-xvf", imageUrl, "-C", unTarFolderUrl).CombinedOutput(); err != nil {
			zaplog.L().Error("untar error", zap.String("file", unTarFolderUrl), zap.Error(err))
			return err
		}
	}
	return nil
}

// 为创建一个文件夹作为容器唯一的可写层
func CreateWriteLayer(containerName string) {
	writeURL := filepath.Join(WriteLayerUrl, containerName)
	if err := os.Mkdir(writeURL, 0777); err != nil {
		zaplog.L().Sugar().Errorf("mkdir %s fail. error: %v", writeURL, err)
	}
}

func CreateMountPoint(imageName, containerName string) error {
	mntUrl := filepath.Join(MntUrl, containerName)
	// 创建 mnt 文件夹作为挂载点
	if err := os.Mkdir(mntUrl, 0777); err != nil {
		zaplog.L().Sugar().Errorf("mkdir %s fail. error: %v", mntUrl, err)
		return err
	}
	tmpWriteLayer := filepath.Join(WriteLayerUrl, containerName)
	tmpImageLocation := filepath.Join(RootUrl, imageName)
	dirs := "dirs=" + tmpWriteLayer + ":" + tmpImageLocation
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", mntUrl).CombinedOutput()
	if err != nil {
		zaplog.L().Error("mount layer cmd error", zap.Error(err))
		return err
	}
	return nil
}

// Delete the AUFS filesystem while container exit
func DeleteWorkSpace(volume, containerName string) {
	volumeUrls := volumeUrlExtract(volume)
	if len(volumeUrls) == 2 && volumeUrls[0] != "" && volumeUrls[1] != "" {
		DeleteMountPointWithVolume(volumeUrls, containerName)
	} else {
		DeleteMountPoint(containerName)
	}

	DeleteWriteLayer(containerName)
}

func DeleteMountPoint(containerName string) error {
	mntUrl := filepath.Join(MntUrl, containerName)
	_, err := exec.Command("umount", mntUrl).CombinedOutput()
	if err != nil {
		zaplog.L().Error("umount cmd error", zap.String("path", mntUrl), zap.Error(err))
		return err
	}
	if err := os.RemoveAll(mntUrl); err != nil {
		zaplog.L().Error("Remove dir error", zap.String("path", mntUrl), zap.Error(err))
		return err
	}
	return nil
}

func DeleteMountPointWithVolume(volumeUrls []string, containerName string) error {
	// 卸载容器里 volume 挂载点的文件系统
	mntUrl := filepath.Join(MntUrl, containerName)
	volumeUrl := filepath.Join(mntUrl, volumeUrls[1])
	_, err := exec.Command("umount", volumeUrl).CombinedOutput()
	if err != nil {
		zaplog.L().Error("umount volume failed.", zap.String("path", volumeUrl), zap.Error(err))
		return err
	}
	// 卸载整个容器文件系统的挂载点
	return DeleteMountPoint(containerName)
}

func DeleteWriteLayer(containerName string) {
	writeURL := filepath.Join(WriteLayerUrl, containerName)
	if err := os.RemoveAll(writeURL); err != nil {
		zaplog.L().Error("Remove dir error", zap.String("path", writeURL), zap.Error(err))
	}
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func volumeUrlExtract(volume string) []string {
	return strings.Split(volume, ":")
}

func MountVolume(volumeURLs []string, containerName string) error {
	// 创建宿主机文件目录
	parentUrl := volumeURLs[0]
	if err := os.Mkdir(parentUrl, 0777); err != nil {
		zaplog.L().Error("mkdir parent dir error", zap.String("path", parentUrl), zap.Error(err))
	}
	// 在容器文件系统里创建挂载点
	volumeUrl := volumeURLs[1]
	containerVolumeUrl := filepath.Join(MntUrl, containerName, volumeUrl)
	if err := os.Mkdir(containerVolumeUrl, 0777); err != nil {
		zaplog.L().Error("mkdir container dir error", zap.String("path", containerVolumeUrl), zap.Error(err))
	}
	// 把宿主机文件目录挂载到容器挂载点
	dirs := "dirs=" + parentUrl
	_, err := exec.Command("mount", "-t", "aufs", "-o", dirs, "none", containerVolumeUrl).CombinedOutput()
	if err != nil {
		zaplog.L().Error("mount volume failed.", zap.Error(err))
		return err
	}
	return nil
}
