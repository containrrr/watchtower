import { useEffect } from "react";
import logo from "../assets/logo.png"

interface SpinnerModalProps {
    visible: boolean;
    title: string;
    message: string;
}

const SpinnerModal = (props: SpinnerModalProps) => {
    useEffect(() => {
        document.body.classList.toggle("modal-open", props.visible === true);
    }, [props.visible])

    if (props.visible !== true) return null;

    return (
        <div>
            <div className="modal-backdrop fade show"></div>
            <div className="modal fade show d-block" tabIndex={-1}>
                <div className="modal-dialog modal-dialog-centered">
                    <div className="modal-content text-center">
                        <div className="modal-body py-5">
                            <div className="d-flex flex-column align-items-center">
                                <div className="w-50 text-center mb-4">
                                    <img src={logo} className="img-fluid" alt="Watchtower logo" />
                                </div>

                                <h5 className="modal-title">{props.title}</h5>
                                <p className="mb-4">{props.message}</p>
                                <div className="spinner-border" role="status">
                                    <span className="visually-hidden">Loading...</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
};

export default SpinnerModal;