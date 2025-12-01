package management

import "github.com/andrelcunha/ottermq/internal/core/models"

func (s *Service) ListVHosts() ([]models.VHostDTO, error) {
	vhosts := s.broker.ListVHosts()
	response := make([]models.VHostDTO, 0, len(vhosts))
	for _, vh := range vhosts {
		dto := models.VHostDTO{
			Name: vh.Name,
		}
		response = append(response, dto)
	}
	return response, nil
}

func (s *Service) GetVHost(name string) (*models.VHostDTO, error) {
	vh := s.broker.GetVHost(name)
	if vh == nil {
		return nil, nil
	}
	dto, err := s.broker.CreateVhostDto(vh)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}
