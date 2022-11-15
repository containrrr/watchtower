import { useEffect, useState } from "react";
import ContainerModel from "../models/ContainerModel";
import { check, list, update } from "../services/Api";
import ContainerList from "./ContainerList";
import Spinner from "./Spinner";
import SpinnerModal from "./SpinnerModal";
import { UpdateSelected, UpdateAll, UpdateCheck } from "./UpdateButtons";

interface ViewModel {
    Containers: ContainerModel[];
}

const ContainerView = () => {
    const [loading, setLoading] = useState(true);
    const [checking, setChecking] = useState(false);
    const [updating, setUpdating] = useState(false);
    const [hasChecked, setHasChecked] = useState(false);
    const [viewModel, setViewModel] = useState<ViewModel>({ Containers: [] });

    const containers = viewModel.Containers;
    const containersWithUpdates = containers.filter((c) => c.HasUpdate);
    const containersWithoutUpdates = containers.filter((c) => !c.HasUpdate);
    const hasSelectedContainers = containers.some((c) => c.Selected);
    const hasUpdates = containersWithUpdates.length > 0;

    const checkForUpdates = async () => {
        setChecking(true);

        setViewModel((m) => ({
            ...m,
            Containers: m.Containers.map((c) => ({
                ...c,
                IsChecking: true
            }))
        }));

        await Promise.all(containers.map(async (c1) => {
            const result = await check(c1.ContainerID);
            setViewModel((m) => ({
                ...m,
                Containers: m.Containers.map((c2) => (c1.ContainerID === c2.ContainerID ? {
                    ...c2,
                    ...result,
                    IsChecking: false
                } : c2
                ))
            }));
        }));

        setChecking(false);
        setHasChecked(true);
    };

    const listContainers = async () => {
        setLoading(true);
        const data = await list();
        setViewModel({
            Containers: data.Containers.map((c) => ({
                ...c,
                Selected: false,
                IsChecking: false,
                HasUpdate: false,
                IsUpdating: false,
                NewVersion: "",
                NewVersionCreated: ""
            }))
        });
        setLoading(false);
        setHasChecked(false);
    };

    const updateImages = async (imagesToUpdate?: string[]) => {
        setUpdating(true);
        await update(imagesToUpdate);
        await listContainers();
        await checkForUpdates();
        setUpdating(false);
    };

    const updateAll = async () => {
        await updateImages();
    };

    const updateSelected = async () => {
        const selectedImages = containers.filter((c) => c.Selected === true).map((c) => c.ImageNameShort);
        await updateImages(selectedImages);
    };

    const onContainerClick = (container: ContainerModel) => {
        setViewModel((m) => ({
            ...m,
            Containers: m.Containers.map((c2) => (container.ContainerID === c2.ContainerID ? {
                ...c2,
                Selected: !c2.Selected,
            } : c2
            ))
        }));
    };

    useEffect(() => {
        listContainers();
    }, []);

    return (
        <main className="mt-5 p-5 d-block">
            <SpinnerModal visible={updating} title="Updating containers" message="Please wait..." />
            <div className="row mb-2">
                <div className="col-12 col-md-4 d-flex align-items-center">
                    {hasUpdates
                        ? <span>{containersWithUpdates.length} container{containersWithUpdates.length === 1 ? " has" : "s have"} updates.</span>
                        : checking
                            ? <span>Checking for updates...</span>
                            : (hasChecked && containers.length > 0)
                                ? <><i className="bi bi-check-circle-fill fs-4 text-primary me-2"></i><span>All containers are up to date.</span></>
                                : <span>{containers.length} running container{containers.length !== 1 && "s"} found.</span>}
                </div>
                <div className="col-12 col-md-8 text-end">
                    {hasUpdates && <UpdateSelected onClick={updateSelected} disabled={checking || !hasSelectedContainers} />}
                    {hasUpdates && <UpdateAll onClick={updateAll} disabled={checking} />}
                    <UpdateCheck onClick={checkForUpdates} disabled={checking} />
                </div>
            </div>

            <ContainerList containers={containersWithUpdates} onContainerClick={onContainerClick} />

            {hasUpdates && containersWithoutUpdates.length > 0 &&
                <div className="row mt-4 mb-2">
                    <div className="col-4 d-flex align-items-center">
                        {containersWithoutUpdates.length} container{containersWithoutUpdates.length === 1 ? " is" : "s are"} up to date.
                    </div>
                </div>
            }

            <ContainerList containers={containersWithoutUpdates} onContainerClick={onContainerClick} />

            {loading && <Spinner />}
        </main>
    );
};

export default ContainerView;