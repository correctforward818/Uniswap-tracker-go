package services

import (
	"fmt"
	"github.com/ausaf007/uniswap-tracker/bindings/erc20"
	"github.com/ausaf007/uniswap-tracker/bindings/uniswap"
	"github.com/ausaf007/uniswap-tracker/models"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"strconv"

	"context"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"os"
	"strings"
)

func loadABI() (string, error) {
	data, err := os.ReadFile("./bindings/uniswap/UniswapV3PoolABI.json")
	if err != nil {
		log.Error(err)
		return "", err
	}
	return string(data), nil
}

type TrackingService struct {
	client   *ethclient.Client
	db       *gorm.DB
	contract *abi.ABI
}

func NewTrackingService(client *ethclient.Client, db *gorm.DB) (*TrackingService, error) {
	uniswapABI, err := loadABI()
	if err != nil {
		return nil, err
	}

	contractABI, err := abi.JSON(strings.NewReader(uniswapABI))
	if err != nil {
		return nil, err
	}

	return &TrackingService{
		client:   client,
		db:       db,
		contract: &contractABI,
	}, nil
}

func (s *TrackingService) Tracker(poolAddress string) error {
	pool := common.HexToAddress(poolAddress)

	// Get the contract instance from pool address
	poolContract, err := uniswap.NewUniswap(pool, s.client)
	if err != nil {
		return fmt.Errorf("error creating pool contract instance: %v", err)
	}

	// Call `slot0` method to get current tick
	slot0Res, err := poolContract.Slot0(nil)
	if err != nil {
		return fmt.Errorf("error calling slot0 method: %v", err)
	}
	tick := slot0Res.Tick

	// Call `token0` and `token1` methods to get token addresses
	token0Address, err := poolContract.Token0(nil)
	if err != nil {
		return fmt.Errorf("error calling token0 method: %v", err)
	}
	token1Address, err := poolContract.Token1(nil)
	if err != nil {
		return fmt.Errorf("error calling token1 method: %v", err)
	}

	// Get the contract instances for each token
	token0Contract, err := erc20.NewErc20(token0Address, s.client)
	if err != nil {
		return fmt.Errorf("error creating token0 contract instance: %v", err)
	}
	token1Contract, err := erc20.NewErc20(token1Address, s.client)
	if err != nil {
		return fmt.Errorf("error creating token1 contract instance: %v", err)
	}

	// Call `balanceOf` method on each token contract to get token balances
	token0Balance, err := token0Contract.BalanceOf(nil, pool)
	if err != nil {
		return fmt.Errorf("error calling balanceOf method on token0: %v", err)
	}
	token1Balance, err := token1Contract.BalanceOf(nil, pool)
	if err != nil {
		return fmt.Errorf("error calling balanceOf method on token1: %v", err)
	}

	blockNumber, err := s.client.BlockNumber(context.Background())
	if err != nil {
		return err
	}

	var poolModel models.Pool
	result := s.db.FirstOrCreate(&poolModel, models.Pool{PoolAddress: poolAddress})
	if result.Error != nil {
		return result.Error
	}

	poolData := &models.PoolData{
		PoolID:        poolModel.ID,
		Token0Balance: token0Balance.String(),
		Token1Balance: token1Balance.String(),
		Tick:          strconv.FormatInt(tick.Int64(), 10),
		BlockNumber:   int64(blockNumber),
	}

	//// TODO: To be removed:
	//fmt.Println("-----Pool_Details-----")
	//fmt.Println("token0Balance:", token0Balance.String())
	//fmt.Println("token1Balance:", token1Balance.String())
	//fmt.Println("Tick:", strconv.FormatInt(tick.Int64(), 10))
	//fmt.Println("Block Number", int64(blockNumber))
	//fmt.Println("-------------")

	log.Info("Pool Details:", poolData)

	// Check for existing record. If it exists, update it. If not, create a new record.
	// This behavior is to account for chain re-organization and to avoid duplicate entries
	var existingData models.PoolData
	if err := s.db.Where("block_number = ? AND pool_id = ?", poolData.BlockNumber, poolData.PoolID).First(&existingData).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Record not found, create a new one
			log.Info("No duplicate record found, creating new record")
			s.db.Create(&poolData)
		} else {
			return err
		}
	} else {
		// Record found, update the existing record
		log.Info("Duplicate record found, updating existing record")
		s.db.Model(&existingData).Updates(models.PoolData{
			Token0Balance: poolData.Token0Balance,
			Token1Balance: poolData.Token1Balance,
			Tick:          poolData.Tick,
		})
	}
	return nil
}

func (s *TrackingService) GetPoolDataOld(poolAddress string, block string) (*models.PoolData, error) {
	var pool models.Pool
	s.db.First(&pool, models.Pool{PoolAddress: poolAddress})

	if block == "latest" {
		var poolData models.PoolData
		result := s.db.Where("pool_id = ?", pool.ID).Order("block_number desc").First(&poolData)

		if result.Error != nil {
			return nil, result.Error
		}

		return &poolData, nil
	} else {
		var poolData models.PoolData
		blockNumber, _ := strconv.ParseInt(block, 10, 64)
		result := s.db.Where("pool_id = ? AND block_number <= ?", pool.ID, blockNumber).Order("block_number desc").First(&poolData)

		if result.Error != nil {
			return nil, result.Error
		}

		return &poolData, nil
	}
}

func (s *TrackingService) GetHistoricPoolDataOld(poolAddress string) ([]models.PoolData, error) {
	var pool models.Pool
	s.db.First(&pool, models.Pool{PoolAddress: poolAddress})

	var poolData []models.PoolData
	result := s.db.Where("pool_id = ?", pool.ID).Order("block_number asc").Find(&poolData)

	if result.Error != nil {
		return nil, result.Error
	}

	return poolData, nil
}

func (s *TrackingService) GetPoolData(poolID uint, block string) (*models.PoolData, error) {
	if block == "latest" {
		var poolData models.PoolData
		result := s.db.Where("pool_id = ?", poolID).Order("block_number desc").First(&poolData)

		if result.Error != nil {
			return nil, result.Error
		}

		return &poolData, nil
	} else {
		var poolData models.PoolData
		blockNumber, _ := strconv.ParseInt(block, 10, 64)
		result := s.db.Where("pool_id = ? AND block_number <= ?", poolID, blockNumber).Order("block_number desc").First(&poolData)

		if result.Error != nil {
			return nil, result.Error
		}

		return &poolData, nil
	}
}

func (s *TrackingService) GetHistoricPoolData(poolID uint) ([]models.PoolData, error) {
	var poolData []models.PoolData
	result := s.db.Where("pool_id = ?", poolID).Order("block_number asc").Find(&poolData)

	if result.Error != nil {
		return nil, result.Error
	}

	return poolData, nil
}
