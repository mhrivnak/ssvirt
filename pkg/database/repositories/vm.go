package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhrivnak/ssvirt/pkg/database/models"
)

type VMRepository struct {
	db *gorm.DB
}

func NewVMRepository(db *gorm.DB) *VMRepository {
	return &VMRepository{db: db}
}

func (r *VMRepository) Create(vm *models.VM) error {
	return r.db.Create(vm).Error
}

func (r *VMRepository) GetByID(id uuid.UUID) (*models.VM, error) {
	var vm models.VM
	err := r.db.Where("id = ?", id).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *VMRepository) GetByVAppID(vappID uuid.UUID) ([]models.VM, error) {
	var vms []models.VM
	err := r.db.Where("vapp_id = ?", vappID).Find(&vms).Error
	return vms, err
}

func (r *VMRepository) GetByVMName(vmName, namespace string) (*models.VM, error) {
	var vm models.VM
	err := r.db.Where("vm_name = ? AND namespace = ?", vmName, namespace).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}

func (r *VMRepository) List() ([]models.VM, error) {
	var vms []models.VM
	err := r.db.Find(&vms).Error
	return vms, err
}

func (r *VMRepository) Update(vm *models.VM) error {
	return r.db.Save(vm).Error
}

func (r *VMRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.VM{}, id).Error
}

func (r *VMRepository) GetWithVApp(id uuid.UUID) (*models.VM, error) {
	var vm models.VM
	err := r.db.Preload("VApp").Where("id = ?", id).First(&vm).Error
	if err != nil {
		return nil, err
	}
	return &vm, nil
}
